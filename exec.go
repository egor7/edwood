package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
)

type Exectab struct {
	name  string
	fn    func(t0, t1, t2 *Text, b0, b1 bool, arg string)
	mark  bool
	flag1 bool
	flag2 bool
}

var exectab = []Exectab{
	//	{ "Abort",		doabort,	false,	true /*unused*/,		true /*unused*/,		},
	{ "Cut",		cut,		true,	true,	true	},
	{"Del", del, false, false, true /*unused*/},
	//	{ "Delcol",		delcol,	false,	true /*unused*/,		true /*unused*/		},
	{"Delete", del, false, true, true /*unused*/},
	//	{ "Dump",		dump,	false,	true,	true /*unused*/		},
	//	{ "Edit",		edit,		false,	true /*unused*/,		true /*unused*/		},
	{"Exit", xexit, false, true /*unused*/, true /*unused*/},
	//	{ "Font",		fontx,	false,	true /*unused*/,		true /*unused*/		},
	//	{ "Get",		get,		false,	true,	true /*unused*/		},
	//	{ "ID",		id,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Incl",		incl,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Indent",		indent,	false,	true /*unused*/,		true /*unused*/		},
	//	{ "Kill",		xkill,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Load",		dump,	false,	false,	true /*unused*/		},
	//	{ "Local",		local,	false,	true /*unused*/,		true /*unused*/		},
	//	{ "Look",		look,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "New",		new,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Newcol",	newcol,	false,	true /*unused*/,		true /*unused*/		},
	//	{ "Paste",		paste,	true,	true,	true /*unused*/		},
	// TODO(rjk): Implement this one.
	//	{ "Put",		put,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Putall",		putall,	false,	true /*unused*/,		true /*unused*/		},
	//	{ "Redo",		undo,	false,	false,	true /*unused*/		},
	//	{ "Send",		sendx,	true,	true /*unused*/,		true /*unused*/		},
	//	{ "Snarf",		cut,		false,	true,	false	},
	//	{ "Sort",		sort,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Tab",		tab,		false,	true /*unused*/,		true /*unused*/		},
	//	{ "Undo",		undo,	false,	true,	true /*unused*/		},
	//	{ "Zerox",		zeroxx,	false,	true /*unused*/,		true /*unused*/		},
}

var wsre = regexp.MustCompile("[ \t\n]+")

func lookup(r string) *Exectab {
	fmt.Println("lookup", r)
	r = wsre.ReplaceAllString(r, " ")
	r = strings.TrimLeft(r, " ")
	r = strings.SplitN(r, " ", 1)[0]
	for _, e := range exectab {
		if e.name == r {
			return &e
		}
	}
	return nil
}

func isexecc(c rune) bool {
	if isfilec(c) {
		return true
	}
	return c == '<' || c == '|' || c == '>'
}

func printarg(argt *Text, q0 int, q1 int) string {
	if argt.what != Body || argt.file.name == "" {
		return ""
	}
	if q0 == q1 {
		return fmt.Sprintf("%s:#%d", argt.file.name, q0)
	} else {
		return fmt.Sprintf("%s:#%d,#%d", argt.file.name, q0, q1)
	}
}

func getarg(argt *Text, doaddr bool, dofile bool) (string, string) {
	if argt == nil {
		return "", ""
	}
	a := ""
	var e Expand
	argt.Commit(true)
	var ok bool
	if e, ok = expand(argt, argt.q0, argt.q1); ok {
		if len(e.name) > 0 && dofile {
			if doaddr {
				a = printarg(argt, e.q0, e.q1)
			}
			return e.name, a
		}
	} else {
		e.q0 = argt.q0
		e.q1 = argt.q1
	}
	n := e.q1 - e.q0
	r := make([]rune, n)
	argt.file.b.Read(e.q0, r)
	if doaddr {
		a = printarg(argt, e.q0, e.q1)
	}
	return string(r), a
}

func execute(t *Text, aq0 int, aq1 int, external bool, argt *Text) {
	Untested()
	var (
		q0, q1 int
		r      []rune
		n, f   int
		dir    string
	)

	q0 = aq0
	q1 = aq1
	if q1 == q0 { // expand to find word (actually file name)
		fmt.Println("q1 == q0")
		// if in selection, choose selection
		if t.inSelection(q0) {
			q0 = t.q0
			q1 = t.q1
			fmt.Println("selection chosen")
		} else {
			for q1 < t.file.b.Nc() {
				c := t.ReadC(q1)
				if isexecc(c) && c != ':' {
					q1++
				} else {
					break
				}
			}
			for q0 > 0 {
				c := t.ReadC(q0 - 1)
				if isexecc(c) && c != ':' {
					q0--
				} else {
					break
				}
			}
			fmt.Println("expanded selection")
			if q1 == q0 {
				fmt.Println("selection chosen")
				return
			}
		}
	}
	r = make([]rune, q1-q0)
	t.file.b.Read(q0, r)
	e := lookup(string(r))
	if !external && t.w != nil && t.w.nopen[QWevent] > 0 {
		f = 0
		if e != nil {
			f |= 1
		}
		if q0 != aq0 || q1 != aq1 {
			r = make([]rune, aq1-aq0)
			t.file.b.Read(aq0, r)
			f |= 2
		}
		aa, a := getarg(argt, true, true)
		if a != "" {
			if len(a) > EVENTSIZE { // too big; too bad
				warning(nil, "argument string too long\n")
				return
			}
			f |= 8
		}
		c := 'x'
		if t.what == Body {
			c = 'X'
		}
		n = aq1 - aq0
		if n <= EVENTSIZE {
			t.w.Event("%c%d %d %d %d %s\n", c, aq0, aq1, f, n, r)
		} else {
			t.w.Event("%c%d %d %d 0 \n", c, aq0, aq1, f, n)
		}
		if q0 != aq0 || q1 != aq1 {
			n = q1 - q0
			r := make([]rune, n)
			t.file.b.Read(q0, r)
			if n <= EVENTSIZE {
				t.w.Event("%c%d %d 0 %d %s\n", c, q0, q1, n, r)
			} else {
				t.w.Event("%c%d %d 0 0 \n", c, q0, q1, n)
			}
		}
		if a != "" {
			t.w.Event("%c0 0 0 %d %s\n", c, len(a), a)
			if aa != "" {
				t.w.Event("%c0 0 0 %d %s\n", c, len(aa), aa)
			} else {
				t.w.Event("%c0 0 0 0 \n", c)
			}
		}
		return
	}
	if e != nil {
		if (e.mark && seltext != nil) && seltext.what == Body {
			seq++
			seltext.w.body.file.Mark()
		}
		s := wsre.ReplaceAllString(string(r), " ")
		s = strings.TrimLeft(s, " ")
		words := strings.SplitN(s, " ", 2)
		if len(words) == 1 {
			words = append(words, "")
		}
		e.fn(t, seltext, argt, e.flag1, e.flag2, words[1])
		return
	}

	b := r
	dir = t.DirName()
	if dir == "." { // sigh
		dir = ""
	}
	a, aa := getarg(argt, true, true)
	if t.w != nil {
		t.w.ref.Inc()
	}
	run(t.w, string(b), dir, true, aa, a, false)
}

func xexit(*Text, *Text, *Text, bool, bool, string) {
	if row.Clean() {
		close(cexit)
		//	threadexits(nil);
	}
}

func del(et *Text, _0 *Text, _1 *Text, flag1 bool, _2 bool, _3 string) {
	fmt.Println("Calling del")
	if et.col == nil || et.w == nil {
		return
	}
	if flag1 || len(et.w.body.file.text) > 1 || et.w.Clean(false) {
		et.col.Close(et.w, true)
	}
}

func cut(et *Text, t *Text, _*Text, dosnarf bool, docut bool, _ string) {
var (
	q0, q1, n, c int
)
	/*
	 * if not executing a mouse chord (et != t) and snarfing (dosnarf)
	 * and executed Cut or Snarf in window tag (et.w != nil),
	 * then use the window body selection or the tag selection
	 * or do nothing at all.
	 */
	if et!=t && dosnarf && et.w!=nil {
		if et.w.body.q1>et.w.body.q0 {
			t = &et.w.body;
			if docut {
				t.file.Mark();	/* seq has been incremented by execute */
			}
		}else{
			 if et.w.tag.q1>et.w.tag.q0 {
				t = &et.w.tag;
			}else{
				t = nil;
			}
		}
	}
	if t == nil {	/* no selection */
		return;
	}
	if t.w!=nil && et.w!=t.w {
		c = 'M';
		if et.w != nil {
			c = et.w.owner;
		}
		t.w.Lock(c);
		defer t.w.Unlock()
	}
	if t.q0 == t.q1 {
		return;
	}
	if dosnarf {
		q0 = t.q0;
		q1 = t.q1;
		snarfbuf.Delete(0, snarfbuf.Nc());
		r := make([]rune, RBUFSIZE)
		for (q0 < q1){
			n = q1 - q0;
			if n > RBUFSIZE {
				n = RBUFSIZE;
			}
			t.file.b.Read(q0, r[:n]);
			snarfbuf.Insert(snarfbuf.Nc(), r[:n]);
			q0 += n;
		}
		acmeputsnarf();
	}
	if docut {
		t.Delete(t.q0, t.q1, true);
		t.SetSelect(t.q0, t.q0);
		if t.w != nil {
			t.ScrDraw();
			t.w.SetTag();
		}
	}else{ 
		if dosnarf 	{/* Snarf command */
			argtext = t;
		}
	}
}

func paste(et *Text, t *Text, _0 *Text, selectall bool, tobody bool, _2 []rune) {
	Unimpl()
}

func get(et *Text, t *Text, argt *Text, flag1 bool, _0 bool, arg []rune) {
	Unimpl()
}
func put(et *Text, _0 *Text, argt *Text, _1 bool, _2 bool, arg []rune) {
	Unimpl()
}

func undo(et *Text, _0 *Text, _1 *Text, flag1 bool, _2 bool, _3 []rune) {
	Unimpl()
}

func run(win *Window, s string, rdir string, newns bool, argaddr string, xarg string, iseditcmd bool) {
	Untested()
	var (
		c    *Command
		cpid chan *os.Process
	)

	if len(s) == 0 {
		return
	}

	c = &Command{}
	cpid = make(chan *os.Process)
	go runproc(win, s, rdir, newns, argaddr, xarg, c, cpid, iseditcmd)
	// This is to avoid blocking waiting for task launch.
	// So runproc sends the resulting process down cpid,
	// and runwait task catches, and records the process in command list (by
	// pumping it down the ccommand chanel)
	go runwaittask(c, cpid)
}

func runwaittask(c *Command, cpid chan *os.Process) {
	c.proc = <-cpid
	c.pid = c.proc.Pid

	if c.pid != 0 { /* successful exec */
		ccommand <- c
	} else {
		if c.iseditcommand {
			cedit <- 0
		}
	}
	cpid = nil
}

func fsopenfd(fsys *client.Fsys, path string, mode uint8) *os.File {
	fid, err := fsys.Open(path, mode)
	if err != nil {
		warning(nil, "Failed to open %v", path)
		return nil
	}

	// open a pipe, serve the reads from fid down it
	r, w, err := os.Pipe()
	if err != nil {
		acmeerror("fsopenfd: Could not make pipe", nil)
	}

	go func() {
		var buf [BUFSIZE]byte
		var werr error
		for {
			n, err := fid.Read(buf[:])
			if n != 0 {
				_, werr = w.Write(buf[0:n])
			}
			if err != nil || werr != nil {
				w.Close()
				return
			}
		}
	}()

	return r
}

func runproc(win *Window, s string, rdir string, newns bool, argaddr string, arg string, c *Command, cpid chan *os.Process, iseditcmd bool) {
	var (
		t, name, filename, dir string
		incl                   []string
		winid                  int
		sfd                    [3]*os.File
		pipechar               int
		//static void *parg[2];
		rcarg []string
		shell string
	)
	Fail := func() {
		// threadexec hasn't happened, so send a zero
		sfd[0].Close()
		if sfd[2] != sfd[1] {
			sfd[2].Close()
		}
		sfd[1].Close()
		cpid <- nil
	}
	Hard := func() {
		//* ugly: set path = (. $cputype /bin)
		//* should honor $path if unusual.
		/* TODO(flux): This looksl ike plan9 magic
		if cputype {
			n = 0;
			memmove(buf+n, ".", 2);
			n += 2;
			i = strlen(cputype)+1;
			memmove(buf+n, cputype, i);
			n += i;
			memmove(buf+n, "/bin", 5);
			n += 5;
			fd = create("/env/path", OWRITE, 0666);
			write(fd, buf, n);
			close(fd);
		}
		*/

		if arg != "" {
			s = fmt.Sprintf("%s '%s'", t, arg) // TODO(flux): BUG: what if quote in arg?
			// This is a bug from the original; and I now know
			// why ' in an argument fails to work properly.
			t = s
			c.text = s
		}
		dir = ""
		if rdir != "" {
			dir = string(rdir)
		}
		shell = acmeshell
		if shell == "" {
			shell = "rc"
		}
		rcarg = []string{shell, "-c", t}
		cmd := exec.Command(rcarg[0], rcarg...)
		cmd.Dir = dir
		cmd.Stdin = sfd[0]
		cmd.Stdout = sfd[1]
		cmd.Stderr = sfd[2]
		if err := cmd.Start(); err == nil {
			if cpid != nil {
				cpid <- cmd.Process
			}
			return // TODO(flux) where do we wait?
		}
		warning(nil, "exec %s: %r\n", shell)
		Fail()
	}
	t = strings.TrimLeft(s, " \t\n")
	name = filepath.Base(string(t)) + " "
	c.name = name
	// t is the full path, trimmed of left whitespace.
	pipechar = 0

	if t[0] == '<' || t[0] == '|' || t[0] == '>' {
		pipechar = int(t[0])
		t = t[1:]
	}
	c.iseditcommand = iseditcmd
	c.text = s
	if newns {
		incl = nil
		if win != nil {
			filename = string(win.body.file.name)
			if len(incl) > 0 {
				incl = make([]string, len(incl)) // Incl is inherited by actions in this window
				for i, inc := range win.incl {
					incl[i] = inc
				}
			}
			winid = win.id
		} else {
			filename = ""
			winid = 0
			if activewin != nil {
				winid = activewin.id
			}
		}
		// 	rfork(RFNAMEG|RFENVG|RFFDG|RFNOTEG); TODO(flux): I'm sure these settings are important

		os.Setenv("winid", fmt.Sprintf("%d", winid))

		if filename != "" {
			os.Setenv("%", filename)
			os.Setenv("samfile", filename)
		}
		c.md = fsysmount(rdir, incl)
		if c.md == nil {
			fmt.Fprintf(os.Stderr, "child: can't allocate mntdir\n")
			return
		}
		fs, err := client.MountService("acme") //, fmt.Sprintf("%d", c.md.id))
		if err == nil {
			fmt.Fprintf(os.Stderr, "child: can't mount acme: %v\n", err)
			fsysdelid(c.md)
			c.md = nil
			return
		}
		if winid > 0 && (pipechar == '|' || pipechar == '>') {
			rdselname := fmt.Sprintf("%d/rdsel", winid)
			sfd[0] = fsopenfd(fs, rdselname, plan9.OREAD)
		} else {
			sfd[0], _ = os.OpenFile("/dev/null", os.O_RDONLY, 0777)
		}
		if (winid > 0 || iseditcmd) && (pipechar == '|' || pipechar == '<') {
			var buf string
			if iseditcmd {
				if winid > 0 {
					buf = fmt.Sprintf("%d/editout", winid)
				} else {
					buf = fmt.Sprintf("editout")
				}
			} else {
				buf = fmt.Sprintf("%d/wrsel", winid)
			}
			sfd[1] = fsopenfd(fs, buf, plan9.OWRITE)
			sfd[2] = fsopenfd(fs, "cons", plan9.OWRITE)
		} else {
			sfd[1] = fsopenfd(fs, "cons", plan9.OWRITE)
			sfd[2] = sfd[1]
		}
		// fsunmount(fs); looks like with plan9.client you just drop it on the floor.
		fs = nil
	} else {
		//	rfork(RFFDG|RFNOTEG);
		fsysclose()
		sfd[0], _ = os.Open("/dev/null")
		sfd[1], _ = os.OpenFile("/dev/null", os.O_WRONLY, 0777)
		nfd, _ := syscall.Dup(erroutfd)
		sfd[2] = os.NewFile(uintptr(nfd), "duped erroutfd")
	}
	if win != nil {
		win.Close()
	}

	if argaddr != "" {
		os.Setenv("acmeaddr", argaddr)
	}
	if acmeshell != "" {
		Hard()
		return
	}
	for _, r := range t {
		if r == ' ' || r == '\t' {
			continue
		}
		if r < ' ' {
			Hard()
			return
		}
		if utfrune([]rune("#;&|^$=`'{}()<>[]*?^~`/"), int(r)) != -1 {
			Hard()
			return
		}
	}

	t = wsre.ReplaceAllString(string(t), " ")
	c.av = strings.Split(t, " ")
	if arg != "" {
		c.av = append(c.av, arg)
	}

	dir = ""
	if rdir != "" {
		dir = string(rdir)
	}
	cmd := exec.Command(c.av[0], c.av[1:]...)
	cmd.Dir = dir
	cmd.Stdin = sfd[0]
	cmd.Stdout = sfd[1]
	cmd.Stderr = sfd[2]
	err := cmd.Start()
	if err == nil {
		if cpid != nil {
			cpid <- cmd.Process
		}
		// Where do we wait TODO(flux)
		return
	}

	Fail()
	return

}
