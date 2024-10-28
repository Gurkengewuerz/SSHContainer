package server

import (
	"context"
	"fmt"
	"github.com/charmbracelet/ssh"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
)

type Server struct {
	config     *Config
	containers *ContainerManager
	log        *logrus.Logger
	sessions   sync.Map // map[string]string - session ID to container ID
}

func New(config *Config, log *logrus.Logger) (*Server, error) {
	containerManager, err := NewContainerManager(config, log)
	if err != nil {
		return nil, err
	}

	return &Server{
		config:     config,
		containers: containerManager,
		log:        log,
	}, nil
}

func (s *Server) authenticateUser(ctx ssh.Context, password string) bool {
	data := url.Values{
		"grant_type":    {"password"},
		"client_id":     {s.config.ClientID},
		"client_secret": {s.config.ClientSecret},
		"username":      {ctx.User()},
		"password":      {password},
	}

	resp, err := http.PostForm(s.config.OAuthEndpoint, data)
	if err != nil {
		s.log.WithFields(logrus.Fields{
			"user":  ctx.User(),
			"error": err,
		}).Error("Authentication request failed")
		return false
	}
	defer resp.Body.Close()

	success := resp.StatusCode == http.StatusOK
	s.log.WithFields(logrus.Fields{
		"user":    ctx.User(),
		"success": success,
	}).Info("Authentication attempt")

	return success
}

func (s *Server) handleSession(sess ssh.Session) {
	ctx := context.Background()
	sessionID := sess.Context().Value(ssh.ContextKeySessionID).(string)

	log := s.log.WithFields(logrus.Fields{
		"user":      sess.User(),
		"sessionID": sessionID,
	})

	log.Info("Starting new session")

	// Get PTY info if available
	ptyReq, winCh, isPty := sess.Pty()

	// Create container config
	containerID, err := s.containers.CreateContainer(ctx, ContainerConfig{
		Image:   s.config.DockerImage,
		Cmd:     sess.Command(),
		Env:     sess.Environ(),
		IsPty:   isPty,
		PtyRows: uint16(ptyReq.Window.Height),
		PtyCols: uint16(ptyReq.Window.Width),
		User:    sess.User(),
	})
	if err != nil {
		log.WithError(err).Error("Failed to create container")
		sess.Exit(1)
		return
	}

	s.sessions.Store(sessionID, containerID)
	cleanup := func() {
		if err := s.containers.RemoveContainer(ctx, containerID); err != nil {
			log.WithError(err).Error("Failed to remove container")
		}
		s.sessions.Delete(sessionID)
	}
	defer cleanup()

	// Start container
	if err := s.containers.StartContainer(ctx, containerID); err != nil {
		log.WithError(err).Error("Failed to start container")
		sess.Exit(1)
		return
	}

	// Attach to container
	stream, err := s.containers.AttachContainer(ctx, containerID)
	if err != nil {
		log.WithError(err).Error("Failed to attach to container")
		sess.Exit(1)
		return
	}
	defer stream.Close()

	// Handle window size changes if PTY was requested
	if isPty {
		go func() {
			for win := range winCh {
				if err := s.containers.ResizeContainer(ctx, containerID, uint16(win.Height), uint16(win.Width)); err != nil {
					log.WithError(err).Error("Failed to resize container")
					break
				}
			}
		}()
	}

	// Setup I/O
	outputErr := make(chan error, 1)
	go func() {
		var err error
		if isPty {
			_, err = io.Copy(sess, stream.Reader)
		} else {
			_, err = stdcopy.StdCopy(sess, sess.Stderr(), stream.Reader)
		}
		outputErr <- err
	}()

	go func() {
		defer stream.CloseWrite()
		io.Copy(stream.Conn, sess)
	}()

	// Wait for container
	statusCh, errCh := s.containers.WaitContainer(ctx, containerID)
	var status int64 = 255

	for {
		select {
		case err = <-errCh:
			if err != nil {
				log.WithError(err).Error("Error waiting for container")
				sess.Exit(1)
				return
			}
		case result := <-statusCh:
			status = result.StatusCode
			log.WithField("status", status).Info("Container exited")
			return
		case <-sess.Context().Done():
			log.Info("Session timeout")
			sess.Exit(1)
			return
		}
	}
}

func (s *Server) Run() error {
	pemBytes, err := os.ReadFile(s.config.SSHHostKey)
	if err != nil {
		return fmt.Errorf("failed to read host key: %w", err)
	}

	signer, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		return fmt.Errorf("failed to parse host key: %w", err)
	}

	server := &ssh.Server{
		Addr:            fmt.Sprintf(":%s", s.config.SSHPort),
		Handler:         s.handleSession,
		HostSigners:     []ssh.Signer{signer},
		PasswordHandler: s.authenticateUser,
		ConnCallback:    nil,
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": func(s ssh.Session) {
				s.Exit(0)
			},
		},
	}

	return server.ListenAndServe()
}
