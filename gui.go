package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anasopking/usermix/mix"
)

// gui serves a small page on localhost that asks for the usernames file and
// how many to generate, then writes generated_<count>.txt like interactive
// mode does, showing progress in the browser.
func gui(args []string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s := &guiServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.page)
	mux.HandleFunc("/start", s.start)
	mux.HandleFunc("/status", s.status)

	url := "http://" + ln.Addr().String() + "/"
	fmt.Println("usermix is open at", url)
	fmt.Println("Close this window to quit.")
	openBrowser(url)
	return http.Serve(ln, mux)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

type guiServer struct {
	mu      sync.Mutex
	running bool
	found   uint64
	total   uint64
	message string
	failed  bool
}

func (s *guiServer) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, guiPage)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *guiServer) start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	fail := func(msg string) { writeJSON(w, map[string]string{"error": msg}) }

	path := strings.Trim(strings.TrimSpace(r.FormValue("file")), `"'`)
	n, err := strconv.ParseUint(strings.TrimSpace(r.FormValue("count")), 10, 64)
	if err != nil || n == 0 {
		fail("Enter a whole number above 0.")
		return
	}
	names, err := readNames(path)
	if err != nil {
		fail("Can't read that file: " + err.Error())
		return
	}
	if len(names) == 0 {
		fail("That file has no usernames in it.")
		return
	}
	corpus := mix.Learn(names, false)
	if corpus.Possible() == 0 {
		fail("Found no usable patterns in that file; try one with more usernames.")
		return
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		fail("Already generating; wait for it to finish.")
		return
	}
	s.running, s.failed, s.found, s.total = true, false, 0, n
	s.message = fmt.Sprintf("Learned from %s usernames; %s possible from them.",
		formatCount(len(names)), formatShort(corpus.Possible()))
	s.mu.Unlock()

	go s.run(corpus, n)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *guiServer) run(corpus *mix.Corpus, n uint64) {
	written, path, elapsed, err := func() (uint64, string, time.Duration, error) {
		tmp, err := os.CreateTemp(".", ".usermix-*.tmp")
		if err != nil {
			return 0, "", 0, err
		}
		defer os.Remove(tmp.Name()) // no-op once renamed
		bw := bufio.NewWriterSize(tmp, 1<<20)

		start := time.Now()
		written := mix.Generate(corpus, n, mix.Options{
			Progress: func(found, _ uint64) {
				s.mu.Lock()
				s.found = found
				s.mu.Unlock()
			},
		}, func(name string) {
			bw.WriteString(name)
			bw.WriteByte('\n')
		})
		if err := bw.Flush(); err != nil {
			tmp.Close()
			return 0, "", 0, err
		}
		if err := tmp.Close(); err != nil {
			return 0, "", 0, err
		}
		path, err := placeNumbered(tmp.Name(), "generated", written)
		return written, path, time.Since(start), err
	}()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if err != nil {
		s.failed = true
		s.message = "Something went wrong: " + err.Error()
		return
	}
	s.found = written
	s.message = fmt.Sprintf("Wrote %s usernames to %s in %s.",
		formatCount(written), path, elapsed.Round(100*time.Millisecond))
	if written < n {
		s.message += " That's fewer than asked for; give usermix more or more varied usernames for a bigger pool."
	}
}

func (s *guiServer) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, map[string]any{
		"running": s.running,
		"found":   s.found,
		"total":   s.total,
		"message": s.message,
		"failed":  s.failed,
	})
}

const guiPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>usermix</title>
<style>
  body { font-family: system-ui, sans-serif; background: #f6f7f9; color: #1d2330; margin: 0; }
  main { max-width: 420px; margin: 48px auto; padding: 28px; background: #fff; border-radius: 12px; box-shadow: 0 2px 12px rgba(0,0,0,.08); }
  h1 { font-size: 22px; margin: 0 0 20px; }
  label { display: block; font-size: 14px; font-weight: 600; margin: 14px 0 6px; }
  input { width: 100%; box-sizing: border-box; padding: 10px; font-size: 15px; border: 1px solid #c8ccd4; border-radius: 8px; }
  button { margin-top: 20px; width: 100%; padding: 11px; font-size: 15px; font-weight: 600; color: #fff; background: #2f6fed; border: 0; border-radius: 8px; cursor: pointer; }
  button:disabled { background: #9db6ea; cursor: default; }
  .bar { margin-top: 20px; height: 10px; background: #e4e7ec; border-radius: 5px; overflow: hidden; }
  .fill { height: 100%; width: 0; background: #2f6fed; transition: width .2s; }
  #msg { margin-top: 14px; font-size: 14px; min-height: 1.4em; }
  #msg.error { color: #c62828; }
</style>
</head>
<body>
<main>
  <h1>usermix</h1>
  <form id="form">
    <label for="file">Usernames file</label>
    <input id="file" name="file" placeholder="e.g. names.txt" required>
    <label for="count">How many usernames</label>
    <input id="count" name="count" type="number" min="1" required>
    <button id="go" type="submit">Generate</button>
  </form>
  <div class="bar"><div class="fill" id="fill"></div></div>
  <div id="msg"></div>
</main>
<script>
const form = document.getElementById("form");
const btn = document.getElementById("go");
const fill = document.getElementById("fill");
const msg = document.getElementById("msg");
function show(text, error) { msg.textContent = text || ""; msg.className = error ? "error" : ""; }
form.addEventListener("submit", async (e) => {
  e.preventDefault();
  btn.disabled = true;
  fill.style.width = "0";
  try {
    const res = await fetch("start", { method: "POST", body: new URLSearchParams(new FormData(form)) });
    const data = await res.json();
    if (data.error) { show(data.error, true); btn.disabled = false; return; }
    poll();
  } catch (err) { show("Lost contact with usermix: " + err, true); btn.disabled = false; }
});
async function poll() {
  try {
    const s = await (await fetch("status")).json();
    fill.style.width = (s.total ? Math.min(100, 100 * s.found / s.total) : 0) + "%";
    show(s.message, s.failed);
    if (s.running) { setTimeout(poll, 300); } else { btn.disabled = false; }
  } catch (err) { show("Lost contact with usermix: " + err, true); btn.disabled = false; }
}
</script>
</body>
</html>
`
