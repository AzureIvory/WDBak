(function () {
  const $ = function (id) {
    return document.getElementById(id);
  };

  function msgSet(txt, ok) {
    const m = $("msg");
    m.textContent = txt || "";
    m.className = "msg" + (ok == null ? "" : ok ? " ok" : " err");
  }

  function defFill() {
    if (typeof goDef !== "function") {
      return;
    }
    goDef()
      .then(function (p) {
        if ($("exe").value.trim() === "") {
          $("exe").value = p || "";
        }
      })
      .catch(function (e) {
        console.error(e);
      });
  }

  function cfgGet() {
    const tStr = $("thr").value.trim();
    let thr = parseInt(tStr, 10);
    if (!thr || thr < 1) thr = 4;

    const raw = $("list").value.split(/\r?\n/);
    const lst = [];
    for (let i = 0; i < raw.length; i++) {
      const v = raw[i].trim();
      if (v) lst.push(v);
    }

    return {
      url: $("url").value.trim(),
      user: $("user").value.trim(),
      pass: $("pass").value.trim(),
      root: $("root").value.trim(),
      mode: $("mode").value,
      thr: thr,
      typ: $("typ").value,
      list: lst,
    };
  }

  function onSave(e) {
    e.preventDefault();

    if (typeof goSave !== "function") {
      msgSet("Go绑定未就绪", false);
      return;
    }

    const cfg = cfgGet();
    if (!cfg.url) {
      msgSet("服务地址不能为空", false);
      return;
    }
    if (!cfg.list.length) {
      msgSet("备份路径至少一个", false);
      return;
    }

    const bak = $("exe").value.trim();
    msgSet("写入中...", true);

    goSave(cfg, bak)
      .then(function (out) {
        if (!out) {
          msgSet("写入成功", true);
        } else {
          msgSet("写入成功: " + out, true);
        }
      })
      .catch(function (err) {
        console.error(err);
        msgSet("写入失败: " + err, false);
      });
  }

  function onClr() {
    if (typeof goClr !== "function") {
      msgSet("Go绑定未就绪", false);
      return;
    }
    const bak = $("exe").value.trim();
    msgSet("处理中...", true);

    goClr(bak)
      .then(function (out) {
        if (!out) {
          msgSet("清空配置成功", true);
        } else {
          msgSet("清空配置成功: " + out, true);
        }
      })
      .catch(function (err) {
        console.error(err);
        msgSet("清空失败: " + err, false);
      });
  }

  function onReset() {
    $("url").value = "";
    $("user").value = "";
    $("pass").value = "";
    $("root").value = "";
    $("mode").value = "skip";
    $("typ").value = "dav";
    $("thr").value = "4";
    $("list").value = "";
    msgSet("", null);
    defFill();
  }

  window.addEventListener("load", function () {
    $("cfg_form").addEventListener("submit", onSave);
    $("btn_reset").addEventListener("click", onReset);
    $("btn_clr").addEventListener("click", onClr);

    // 默认值
    $("root").value = "";
    $("mode").value = "skip";
    $("typ").value = "dav";
    $("thr").value = "4";

    defFill();
  });
})();
