package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Job struct {
	L string
	R string
}

type DirC struct {
	mu  sync.Mutex
	set map[string]struct{}
}

var stTot int64
var stSkp int64
var stOk int64
var stErr int64

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx); err != nil {
		if ctx.Err() != nil {
			log.Printf("stop: %v\n", ctx.Err())
		} else {
			log.Printf("err: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe = filepath.Clean(exe)
	dir := filepath.Dir(exe)

	cfg, err := rdCfg(exe)
	if err != nil {
		return err
	}

	setDbg(cfg.Debug)

	if cfg.Thr <= 0 {
		cfg.Thr = runtime.NumCPU()
		if cfg.Thr < 1 {
			cfg.Thr = 1
		}
	}

	log.Printf("thr=%d mode=%s\n", cfg.Thr, cfg.Mode)

	dc := &DirC{set: make(map[string]struct{})}
	q := make(chan Job, cfg.Thr*4)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Thr; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sto, err := mkSto(cfg)
			if err != nil {
				log.Printf("[ERR] mkSto: %v\n", err)
				return
			}
			defer sto.Cls()
			for j := range q {
				if ctx.Err() != nil {
					return
				}
				if err := upOne(ctx, sto, cfg, j, dc); err != nil {
					atomic.AddInt64(&stErr, 1)
					log.Printf("[ERR] %v\n", err)
				}
			}
		}()
	}

	for _, s := range cfg.List {
		if ctx.Err() != nil {
			break
		}
		if !filepath.IsAbs(s) {
			s = filepath.Join(dir, s)
		}
		if err := addJob(ctx, s, q); err != nil {
			log.Printf("[ERR] add %s: %v\n", s, err)
		}
	}

	close(q)
	wg.Wait()

	log.Printf("tot=%d ok=%d skip=%d err=%d\n",
		atomic.LoadInt64(&stTot),
		atomic.LoadInt64(&stOk),
		atomic.LoadInt64(&stSkp),
		atomic.LoadInt64(&stErr),
	)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func addJob(ctx context.Context, src string, q chan<- Job) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	src = filepath.Clean(src)
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		base := filepath.Base(src)
		return filepath.WalkDir(src, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rel, e := filepath.Rel(src, p)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			rp := base
			if rel != "." {
				rp = path.Join(base, rel)
			}
			q <- Job{L: p, R: rp}
			return nil
		})
	}

	if !st.Mode().IsRegular() {
		return nil
	}
	base := filepath.Base(src)
	q <- Job{L: src, R: base}
	return nil
}

func upOne(ctx context.Context, sto Sto, cfg *Cfg, j Job, dc *DirC) error {
	st, err := os.Stat(j.L)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return nil
	}

	atomic.AddInt64(&stTot, 1)

	rem := j.R
	if cfg.Root != "" {
		rem = path.Join(cfg.Root, rem)
	}

	dp := path.Dir(rem)
	if dp != "." && dp != "/" {
		if err := mkDir(ctx, sto, dc, dp); err != nil {
			return fmt.Errorf("mkDir %s: %w", dp, err)
		}
	}

	if cfg.Mode == "skip" {
		ok, err := sto.Has(ctx, rem)
		if err != nil {
			return fmt.Errorf("has %s: %w", rem, err)
		}
		if ok {
			atomic.AddInt64(&stSkp, 1)
			log.Printf("[SKIP] %s -> %s\n", j.L, rem)
			return nil
		}
	}

	if err := sto.Put(ctx, j.L, rem, st.Size()); err != nil {
		return err
	}
	atomic.AddInt64(&stOk, 1)
	return nil
}

func mkDir(ctx context.Context, sto Sto, dc *DirC, dp string) error {
	dp = strings.Trim(dp, "/")
	if dp == "" {
		return nil
	}
	seg := strings.Split(dp, "/")
	cur := ""
	for _, s := range seg {
		if s == "" {
			continue
		}
		if cur == "" {
			cur = s
		} else {
			cur = cur + "/" + s
		}

		dc.mu.Lock()
		_, ok := dc.set[cur]
		dc.mu.Unlock()
		if ok {
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := sto.Mk(ctx, cur); err != nil {
			return err
		}

		dc.mu.Lock()
		dc.set[cur] = struct{}{}
		dc.mu.Unlock()
	}
	return nil
}
