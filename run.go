package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// cmdRun starts Matriline's server and client for this project and keeps them running:
// one that ends unexpectedly is started again; a change of the client's settings (config
// apply, the web page) restarts the client, which reads them when it starts (running
// calculations continue from their last checkpoint). Ctrl-C or the service manager stops
// both.
func cmdRun(c *conf) error {
	p := pathsOf(c)
	if _, err := apply(c); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logf("Nacomline %s, project %s", version, p.project)

	srv := &child{name: "server", p: p, args: []string{"run"}, log: filepath.Join(p.hidden, "server.out")}
	if err := srv.start(); err != nil {
		return err
	}
	defer srv.stop()
	logf("waiting for the server (the first start checks the ORCA installation: a minute or two)")
	if err := waitServer(ctx, p, srv); err != nil {
		return err
	}
	if err := ensureCredential(p); err != nil {
		return err
	}
	cli := &child{name: "client", p: p, args: []string{"run"}, log: filepath.Join(p.hidden, "client.out")}
	if err := cli.start(); err != nil {
		return err
	}
	defer cli.stop()
	logf("running: put inputs in %s; 'nacomline status' or 'nacomline watch' to follow", filepath.Join(p.project, "input"))

	lastClientConf := fileStamp(p.client)
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			logf("stopping")
			return nil
		case <-tick.C:
		}
		if srv.exited() {
			logf("the server ended (%v); starting it again", srv.err)
			time.Sleep(5 * time.Second)
			if err := srv.start(); err != nil {
				logf("server: %v", err)
			}
		}
		if s := fileStamp(p.client); s != lastClientConf {
			lastClientConf = s
			logf("the client's settings changed: restarting it")
			cli.stop()
		}
		if cli.exited() {
			if cli.err != nil {
				logf("the client ended (%v); starting it again", cli.err)
				time.Sleep(5 * time.Second)
			}
			if err := cli.start(); err != nil {
				logf("client: %v", err)
			}
		}
	}
}

func logf(format string, a ...any) {
	fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}

func fileStamp(p string) string {
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano())
}

// child is one Matriline process run by cmdRun.
type child struct {
	name string
	p    paths
	args []string
	log  string
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func (ch *child) start() error {
	cmd := matriline(ch.p, ch.name, ch.args...)
	f, err := os.OpenFile(ch.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, f, f
	if ch.name == "server" {
		cmd.Dir = ch.p.project
	} else {
		cmd.Dir = ch.p.clientDir
	}
	withParent(cmd)
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("starting matriline-%s: %v", ch.name, err)
	}
	keepWithParent(cmd)
	ch.cmd, ch.done, ch.err = cmd, make(chan struct{}), nil
	go func() {
		ch.err = cmd.Wait()
		f.Close()
		close(ch.done)
	}()
	return nil
}

func (ch *child) exited() bool {
	if ch.done == nil {
		return true
	}
	select {
	case <-ch.done:
		return true
	default:
		return false
	}
}

// stop ends the process: the server with its own 'stop' (it saves its state), the client
// with an interrupt (Unix) or ended (Windows: no interrupt for another process; running
// calculations continue from their checkpoint at the next start).
func (ch *child) stop() {
	if ch.exited() {
		return
	}
	if ch.name == "server" {
		capture(ch.p, "server", "stop")
	} else if runtime.GOOS != "windows" {
		ch.cmd.Process.Signal(os.Interrupt)
	} else {
		ch.cmd.Process.Kill()
	}
	select {
	case <-ch.done:
	case <-time.After(60 * time.Second):
		ch.cmd.Process.Kill()
		<-ch.done
	}
	ch.err = nil // asked for
}

func waitServer(ctx context.Context, p paths, srv *child) error {
	for {
		if _, err := capture(p, "server", "version"); err == nil {
			if out, _ := capture(p, "server", "status"); strings.HasPrefix(out, "server ") {
				return nil
			}
		}
		if srv.exited() {
			b, _ := os.ReadFile(srv.log)
			return fmt.Errorf("the server did not start (%v); its last words:\n%s", srv.err, tail(string(b), 1500))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// ensureCredential gives the local client its one-time credential the first time.
func ensureCredential(p paths) error {
	if fileExists(filepath.Join(p.clientDir, "state", "client.key")) || fileExists(p.cred) {
		return nil
	}
	out, err := capture(p, "server", "keys", "issue", "this-computer", p.cred)
	if err != nil && strings.Contains(out, "already issued") {
		capture(p, "server", "keys", "revoke", "this-computer") // an unused one from an interrupted first start
		out, err = capture(p, "server", "keys", "issue", "this-computer", p.cred)
	}
	if err != nil {
		return fmt.Errorf("issuing the local client's credential: %s", out)
	}
	return nil
}
