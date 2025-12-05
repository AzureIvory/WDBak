package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type DavSto struct {
	url string
	usr string
	pas string
	cli *http.Client
}

func newDav(cfg *Cfg) (Sto, error) {
	cli := mkCli(cfg.Thr)
	u := strings.TrimRight(cfg.Url, "/")
	return &DavSto{
		url: u,
		usr: cfg.User,
		pas: cfg.Pass,
		cli: cli,
	}, nil
}

func mkCli(thr int) *http.Client {
	if thr < 1 {
		thr = 1
	}
	tr := &http.Transport{
		MaxIdleConns:        thr * 4,
		MaxIdleConnsPerHost: thr * 2,
		MaxConnsPerHost:     thr * 2,
		DisableCompression:  true,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   0,
	}
}

func (d *DavSto) Cls() {
	if tr, ok := d.cli.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (d *DavSto) Mk(ctx context.Context, dir string) error {
	if dir == "" {
		return nil
	}
	u := mkURL(d.url, dir)

	return doTry(ctx, 3, func() error {
		req, err := http.NewRequestWithContext(ctx, "MKCOL", u, nil)
		if err != nil {
			return err
		}
		if d.usr != "" {
			req.SetBasicAuth(d.usr, d.pas)
		}
		resp, err := d.cli.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == 405 || resp.StatusCode == 301 ||
			resp.StatusCode == 302 || resp.StatusCode == 307 ||
			resp.StatusCode == 308 {
			return nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("mkcol %s: %s", u, resp.Status)
		}
		return nil
	})
}

func (d *DavSto) Has(ctx context.Context, rem string) (bool, error) {
	u := mkURL(d.url, rem)

	var ok bool
	err := doTry(ctx, 3, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
		if err != nil {
			return err
		}
		if d.usr != "" {
			req.SetBasicAuth(d.usr, d.pas)
		}
		resp, err := d.cli.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			ok = false
			return nil
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			ok = true
			return nil
		}
		return fmt.Errorf("head %s: %s", u, resp.Status)
	})
	return ok, err
}

func (d *DavSto) Put(ctx context.Context, loc, rem string, sz int64) error {
	u := mkURL(d.url, rem)
	dbgLogf("[DBG] PUT %s -> %s (%d bytes)", loc, u, sz)
	return doTry(ctx, 3, func() error {
		f, err := os.Open(loc)
		if err != nil {
			return err
		}
		defer f.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, f)
		if err != nil {
			return err
		}
		if d.usr != "" {
			req.SetBasicAuth(d.usr, d.pas)
		}
		req.ContentLength = sz

		t0 := time.Now()
		resp, err := d.cli.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("put %s: %s", u, resp.Status)
		}

		dur := time.Since(t0).Seconds()
		if dur <= 0 {
			dur = 0.001
		}
		mb := float64(sz) / 1024.0 / 1024.0
		spd := mb / dur
		dbgLogf("[OK ] %s -> %s (%.2f MB, %.1fs, %.2f MB/s)\n",
			loc, u, mb, dur, spd)
		return nil
	})
}

func mkURL(base, rp string) string {
	b := strings.TrimRight(base, "/")
	p := strings.TrimLeft(rp, "/")
	if p == "" {
		return b + "/"
	}
	seg := strings.Split(p, "/")
	for i, s := range seg {
		seg[i] = url.PathEscape(s)
	}
	return b + "/" + strings.Join(seg, "/")
}
