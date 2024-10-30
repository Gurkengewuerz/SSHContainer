package server

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/ssh"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
)

type Server struct {
	config     *Config
	containers *ContainerManager
	log        *logrus.Logger
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
	username := sess.User()

	log := s.log.WithFields(logrus.Fields{
		"user":      username,
		"sessionID": sessionID,
	})

	log.Info("Starting new session")

	// Get PTY info if available
	ptyReq, winCh, isPty := sess.Pty()

	// Get or create container for user
	containerID, err := s.containers.GetOrCreateContainer(ctx, username, sess.Environ())
	if err != nil {
		log.WithError(err).Error("Failed to get or create container")
		sess.Exit(1)
		return
	}
	defer s.containers.ReleaseContainer(username)

	var stream types.HijackedResponse
	var execID string

	// Attach to container
	cmd := s.config.ContainerCMD
	if len(sess.Command()) > 0 {
		cmd = sess.Command()
	}
	// Execute specific command
	stream, execID, err = s.containers.ExecInContainer(ctx, containerID, sess.Environ(), cmd, s.config.ContainerUser, isPty)
	if err != nil {
		log.WithError(err).Error("Failed to exec in container")
		sess.Exit(1)
		return
	}

	defer stream.Close()

	// Handle window size changes if PTY was requested
	if isPty {
		go func() {
			for win := range winCh {
				var err error
				if execID != "" {
					err = s.containers.ResizeExec(ctx, execID, uint16(win.Height), uint16(win.Width))
				} else {
					err = s.containers.client.ContainerResize(ctx, containerID, container.ResizeOptions{
						Height: uint(win.Height),
						Width:  uint(win.Width),
					})
				}
				if err != nil {
					log.WithError(err).Error("Failed to resize")
				}
			}
		}()

		// Set initial terminal size
		if execID != "" {
			err = s.containers.ResizeExec(ctx, execID, uint16(ptyReq.Window.Height), uint16(ptyReq.Window.Width))
		} else {
			err = s.containers.client.ContainerResize(ctx, containerID, container.ResizeOptions{
				Height: uint(ptyReq.Window.Height),
				Width:  uint(ptyReq.Window.Width),
			})
		}
		if err != nil {
			log.WithError(err).Error("Failed to set initial terminal size")
		}
	}

	// Setup I/O copying
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

	defer func() {
		log.Info("Session ended")
	}()
	// Wait for either the session to end or an error to occur
	select {
	case err := <-outputErr:
		if err != nil {
			log.WithError(err).Error("Error in I/O copy")
			sess.Exit(1)
			return
		}
	case <-sess.Context().Done():
		log.Info("Session timeout")
		return
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

	forwardHandler := &ssh.ForwardedTCPHandler{}
	server := &ssh.Server{
		Addr:            fmt.Sprintf(":%s", s.config.SSHPort),
		Handler:         s.handleSession,
		HostSigners:     []ssh.Signer{signer},
		PasswordHandler: s.authenticateUser,
		ConnCallback:    nil,
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": func(sess ssh.Session) {
				defer sess.Close()
				s.log.WithFields(logrus.Fields{
					"user": sess.User(),
				}).Warn("SFTP subsystem is disabled")
				sess.Exit(0)
			},
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			s.log.Warn("attempt to bind", dhost, dport, "denied")
			return false
		}),
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			s.log.Warn("attempt to bind", host, port, "denied")
			return false
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
	}

	// Set up signal handling for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		s.log.Info("Caught interrupt signal, cleaning up")
		s.containers.Shutdown()
		os.Exit(0)
	}()

	s.log.WithField("port", s.config.SSHPort).Info("Starting SSH server")
	return server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down server")
	s.containers.Shutdown()
	return nil
}
