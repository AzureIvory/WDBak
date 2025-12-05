package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	webview "github.com/webview/webview_go"
)

type Cfg struct {
	Url   string   `json:"url"`
	User  string   `json:"user"`
	Pass  string   `json:"pass"`
	Root  string   `json:"root"`
	Mode  string   `json:"mode"`
	Thr   int      `json:"thr"`
	Typ   string   `json:"typ"`
	List  []string `json:"list"`
	Debug bool     `json:"debug"`
}

const mag = "CFG_TAIL1"

func main() {
	// 有参数时走命令行模式
	if len(os.Args) > 1 {
		if err := cli(os.Args[1:]); err != nil {
			fmt.Println("err:", err)
			os.Exit(1)
		}
		return
	}
	runUI()
}

func cli(a []string) error {
	var bak string
	var cfgp string

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	defBak := filepath.Join(dir, "WDBak.bak")

	if len(a) == 1 {
		bak = defBak
		cfgp = a[0]
	} else if len(a) == 2 {
		bak = a[0]
		cfgp = a[1]
	} else {
		return errors.New("use: cfgtool [bak_path] cfg.json")
	}

	js, err := os.ReadFile(cfgp)
	if err != nil {
		return err
	}

	var c Cfg
	if err := json.Unmarshal(js, &c); err != nil {
		return fmt.Errorf("bad cfg json: %w", err)
	}
	if err := ckCfg(&c); err != nil {
		return err
	}

	js, err = json.MarshalIndent(&c, "", "  ")
	if err != nil {
		return err
	}

	out, err := gen(bak, js)
	if err != nil {
		return err
	}

	fmt.Println("out:", out)
	return nil
}

// GUI 模式
func runUI() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("get exe err:", err)
		return
	}
	dir := filepath.Dir(exe)
	defBak := filepath.Join(dir, "WDBak.bak")
	ui := filepath.Join(dir, "ui", "index.html")

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("WDBak 配置")
	w.SetSize(900, 620, webview.HintFixed)

	// 模板WDBak.bak路径
	_ = w.Bind("goDef", func() (string, error) {
		return defBak, nil
	})

	// 保存配置，从 bak 生成对应系统可执行文件
	_ = w.Bind("goSave", func(c Cfg, bak string) (string, error) {
		if err := ckCfg(&c); err != nil {
			return "", err
		}

		js, err := json.MarshalIndent(&c, "", "  ")
		if err != nil {
			return "", err
		}

		tgt := bak
		if strings.TrimSpace(tgt) == "" {
			tgt = defBak
		}

		out, err := gen(tgt, js)
		if err != nil {
			return "", err
		}
		return out, nil
	})

	// 清空尾巴，从bak生成无尾巴的可执行文件
	_ = w.Bind("goClr", func(bak string) (string, error) {
		tgt := bak
		if strings.TrimSpace(tgt) == "" {
			tgt = defBak
		}
		out, err := clr(tgt)
		if err != nil {
			return "", err
		}
		return out, nil
	})

	p := filepath.ToSlash(ui)
	u := &url.URL{Scheme: "file", Path: "/" + p}
	w.Navigate(u.String())
	w.Run()
}

// 填默认值并简单校验
func ckCfg(c *Cfg) error {
	if c.Url == "" {
		return errors.New("cfg url empty")
	}
	if len(c.List) == 0 {
		return errors.New("cfg list empty")
	}
	if c.Mode == "" {
		c.Mode = "skip"
	}
	if c.Thr <= 0 {
		c.Thr = 4
	}
	if c.Typ == "" {
		c.Typ = "dav"
	}
	return nil
}

// 构造带尾巴的新buf，去掉旧尾巴再加新的
func mkBuf(dat, js []byte) ([]byte, error) {
	if len(js) > 1<<31-1 {
		return nil, fmt.Errorf("cfg too big")
	}

	idx := bytes.LastIndex(dat, []byte(mag))
	head := dat
	if idx >= 0 {
		head = dat[:idx]
	}

	ln := len(js)
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(ln))

	buf := make([]byte, 0, len(head)+len(mag)+4+ln)
	buf = append(buf, head...)
	buf = append(buf, []byte(mag)...)
	buf = append(buf, lb[:]...)
	buf = append(buf, js...)
	return buf, nil
}

// 命令行模式
func wr(exe string, js []byte) error {
	dat, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	buf, err := mkBuf(dat, js)
	if err != nil {
		return err
	}

	tmp := exe + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o755); err != nil {
		return err
	}
	if err := os.Remove(exe); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		return err
	}
	return nil
}

// 根据模板bak生成当前系统可执行文件
func gen(bak string, js []byte) (string, error) {
	dat, err := os.ReadFile(bak)
	if err != nil {
		return "", err
	}
	buf, err := mkBuf(dat, js)
	if err != nil {
		return "", err
	}

	out := outP(bak)
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o755); err != nil {
		return "", err
	}
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(tmp, out); err != nil {
		return "", err
	}
	return out, nil
}

// 根据模板bak生成无尾巴的可执行文件
func clr(bak string) (string, error) {
	dat, err := os.ReadFile(bak)
	if err != nil {
		return "", err
	}

	idx := bytes.LastIndex(dat, []byte(mag))
	head := dat
	if idx >= 0 {
		head = dat[:idx]
	}

	out := outP(bak)
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, head, 0o755); err != nil {
		return "", err
	}
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(tmp, out); err != nil {
		return "", err
	}
	return out, nil
}

// 根据系统选择输出文件名
func outP(bak string) string {
	dir := filepath.Dir(bak)
	base := filepath.Base(bak)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	suf := ""
	if runtime.GOOS == "windows" {
		suf = ".exe"
	}
	return filepath.Join(dir, base+suf)
}
