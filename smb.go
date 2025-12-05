package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

type SmbSto struct {
	fs   *smb2.Share
	ses  *smb2.Session
	con  net.Conn
	root string
}

func newSmb(cfg *Cfg) (Sto, error) {
	host, sh, base, err := prsSmb(cfg.Url)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(host, "445"))
	if err != nil {
		return nil, err
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.User,
			Password: cfg.Pass,
		},
	}

	ses, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	unc := `\\` + host + `\` + sh
	fs, err := ses.Mount(unc)
	if err != nil {
		ses.Logoff()
		conn.Close()
		return nil, err
	}

	r := strings.Trim(base, "/")
	return &SmbSto{
		fs:   fs,
		ses:  ses,
		con:  conn,
		root: r,
	}, nil
}

func (s *SmbSto) Cls() {
	if s.fs != nil {
		_ = s.fs.Umount()
	}
	if s.ses != nil {
		_ = s.ses.Logoff()
	}
	if s.con != nil {
		_ = s.con.Close()
	}
}

func (s *SmbSto) full(rem string) string {
	if s.root == "" {
		return rem
	}
	return path.Join(s.root, rem)
}

func (s *SmbSto) Mk(ctx context.Context, dir string) error {
	if dir == "" {
		return nil
	}
	p := s.full(dir)
	if err := s.fs.MkdirAll(p, 0777); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *SmbSto) Has(ctx context.Context, rem string) (bool, error) {
	p := s.full(rem)
	f, err := s.fs.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	f.Close()
	return true, nil
}

func (s *SmbSto) Put(ctx context.Context, loc, rem string, sz int64) error {
	p := s.full(rem)
	dbgLogf("[DBG] PUT %s -> smb:%s (%d bytes)", loc, p, sz)
	return doTry(ctx, 3, func() error {
		in, err := os.Open(loc)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := s.fs.Create(p)
		if err != nil {
			return err
		}
		defer out.Close()

		t0 := time.Now()
		n, err := io.Copy(out, in)
		if err != nil {
			return err
		}

		dur := time.Since(t0).Seconds()
		if dur <= 0 {
			dur = 0.001
		}
		mb := float64(n) / 1024.0 / 1024.0
		spd := mb / dur
		dbgLogf("[OK ] %s -> smb:%s (%.2f MB, %.1fs, %.2f MB/s)\n",
			loc, p, mb, dur, spd)
		return nil
	})
}

func prsSmb(su string) (host, sh, base string, err error) {
	t := strings.TrimSpace(su)
	if strings.HasPrefix(strings.ToLower(t), "smb://") {
		u, e := url.Parse(t)
		if e != nil {
			err = e
			return
		}
		host = u.Hostname()
		if host == "" {
			err = fmt.Errorf("bad smb url: %s", t)
			return
		}
		p := strings.Trim(u.Path, "/")
		if p == "" {
			err = fmt.Errorf("need share: %s", t)
			return
		}
		seg := strings.SplitN(p, "/", 2)
		sh = seg[0]
		if len(seg) > 1 {
			base = seg[1]
		}
		return
	}

	t = strings.TrimPrefix(t, `\\`)
	seg := strings.SplitN(t, `\`, 3)
	if len(seg) < 2 {
		err = fmt.Errorf("bad smb path: %s", su)
		return
	}
	host = seg[0]
	sh = seg[1]
	if len(seg) == 3 {
		base = strings.ReplaceAll(seg[2], `\`, `/`)
	}
	return
}
