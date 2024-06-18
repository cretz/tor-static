package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	verbose       bool
	host          string
	autopointPath string
	folders       = []string{"_openssl", "libevent", "zlib", "xz", "tor"}
	absCurrDir    = getAbsCurrDir()
	numJobsInt    = runtime.NumCPU()
	numJobs       = ""
)

func main() {
	flag.BoolVar(&verbose, "verbose", false, "Whether to show command output")
	flag.StringVar(&host, "host", "", "Host option, useful for cross-compilation")
	flag.StringVar(&autopointPath, "autopoint-path", "/usr/local/opt/gettext/bin", "OSX: Directory that contains autopoint binary")
	flag.IntVar(&numJobsInt, "j", runtime.NumCPU(), "Number of jobs to run in parallel")
	flag.Parse()
	numJobs = fmt.Sprintf("-j%d", numJobsInt)
	if len(flag.Args()) != 1 {
		log.Fatal("Missing command. Can be build-all, build-<folder>, clean-all, clean-<folder>, show-libs, or package-libs")
	}
	if err := run(flag.Args()[0]); err != nil {
		log.Fatal(err)
	}
}

func run(cmd string) error {
	if err := validateEnvironment(); err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(cmd, "build-"):
		return build(cmd[6:])
	case strings.HasPrefix(cmd, "clean-"):
		return clean(cmd[6:])
	case cmd == "show-libs":
		return showLibs()
	case cmd == "package-libs":
		return packageLibs()
	default:
		return fmt.Errorf("Invalid command: %v. Should be build-all, build-<folder>, clean-all, clean-<folder>, show-libs, or package-libs", cmd)
	}
}

func getAbsCurrDir() string {
	var err error
	absCurrDir, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(absCurrDir)
		absCurrDir = "/" + strings.TrimSuffix(volume, ":") + "/" + filepath.ToSlash(absCurrDir[len(volume)+1:])
	}
	return absCurrDir
}

func validateEnvironment() error {
	// Make sure all the folders are there
	for _, folder := range folders {
		if info, err := os.Stat(folder); err != nil || !info.IsDir() {
			return fmt.Errorf("%v is not a dir", folder)
		}
	}
	switch runtime.GOOS {
	// On windows, have to verify MinGW
	case "windows":
		// Confirm it is MinGW 64
		if byts, err := exec.Command("uname", "-a").CombinedOutput(); err != nil {
			return fmt.Errorf("This has to be run in a MSYS or MinGW shell, uname failed: %v", err)
		} else if !bytes.HasPrefix(byts, []byte("MINGW64")) && !bytes.HasPrefix(byts, []byte("MSYS2")) {
			return fmt.Errorf("This has to be run in a MSYS or MinGW64 shell, uname output: %v", string(byts))
		}
	case "linux":
		// Make sure it's not MinGW
		if byts, err := exec.Command("uname", "-a").CombinedOutput(); err != nil {
			return fmt.Errorf("Failed running uname -a")
		} else if bytes.HasPrefix(byts, []byte("MINGW")) {
			return fmt.Errorf("MinGW should not use Linux Go binary, but instead a Windows go.exe to run the build")
		}
	}
	return nil
}

func build(folder string) error {
	log.Printf("*** Building %v ***", folder)
	defer log.Printf("*** Done building %v ***", folder)
	pwd := absCurrDir + "/" + folder
	switch folder {
	case "all":
		for _, subFolder := range folders {
			if err := build(subFolder); err != nil {
				return err
			}
		}
		return nil
	case "_openssl":
		prefix := pwd + "/dist"
		cmds := [][]string{
			{"sh", "./config", "--prefix=" + prefix, "--openssldir=" + prefix, "no-shared", "no-dso", "no-zlib"},
			{"make", "depend"},
			{"make", numJobs},
			{"make", "install_sw"},
		}
		if runtime.GOOS == "windows" {
			cmds[0] = append(cmds[0], "mingw64")
			cmds[0][0] = "perl"
			cmds[0][1] = "./Configure"
		} else if runtime.GOOS == "darwin" {
			cmds[0][0] = "perl"
			cmds[0][1] = "./Configure"

			byts, err := exec.Command("uname", "-m").CombinedOutput()
			if err != nil {
				return err
			}

			var defaultOSCompiler string
			switch string(byts) {
			case "x86_64\n":
				defaultOSCompiler = "darwin64-x86_64-cc"
			case "arm64\n":
				defaultOSCompiler = "darwin64-arm64-cc"
			}

			if host != "" {
				switch {
				case strings.HasPrefix(host, "x86_64"):
					defaultOSCompiler = "darwin64-x86_64-cc"
				case strings.HasPrefix(host, "arm64"):
					defaultOSCompiler = "darwin64-arm64-cc"
				default:
					return errors.New("unsupported architecture")
				}
			}

			cmds[0] = append(cmds[0], defaultOSCompiler)
		}

		return runCmds(folder, nil, cmds)
	case "libevent":
		cmds := [][]string{
			{"sh", "-l", "./autogen.sh"},
			{"sh", "./configure", "--prefix=" + pwd + "/dist",
				"--disable-shared", "--enable-static", "--with-pic", "--disable-samples", "--disable-libevent-regress",
				"CPPFLAGS=-I../_openssl/dist/include", "LDFLAGS=-L../_openssl/dist/lib"},
			{"make", numJobs},
			{"make", "install"},
		}

		if host != "" {
			cmds[1] = append(cmds[1], "--host="+host)
		}
		return runCmds(folder, nil, cmds)
	case "zlib":
		var env []string
		cmds := [][]string{
			{"sh", "./configure", "--prefix=" + pwd + "/dist", "--static"},
			{"make", numJobs},
			{"make", "install"},
		}
		if runtime.GOOS == "windows" {
			env = []string{"PREFIX=" + pwd + "/dist", "BINARY_PATH=" + pwd + "/dist/bin",
				"INCLUDE_PATH=" + pwd + "/dist/include", "LIBRARY_PATH=" + pwd + "/dist/lib"}
			cmds = [][]string{{"make", "-fwin32/Makefile.gcc"}, {"make", "install", "-fwin32/Makefile.gcc"}}
		}
		return runCmds(folder, env, cmds)
	case "xz":
		var env []string
		if runtime.GOOS == "darwin" {
			env = []string{"PATH=" + autopointPath + ":" + os.Getenv("PATH")}
		}
		cmds := [][]string{
			{"sh", "-l", "./autogen.sh", "--no-po4a"},
			{"sh", "./configure", "--prefix=" + pwd + "/dist", "--disable-shared", "--enable-static",
				"--disable-doc", "--disable-scripts", "--disable-xz", "--disable-xzdec", "--disable-lzmadec",
				"--disable-lzmainfo", "--disable-lzma-links"},
			{"make", numJobs},
			{"make", "install"},
		}
		if host != "" {
			cmds[1] = append(cmds[1], "--host="+host)
		}
		return runCmds(folder, env, cmds)
	case "tor":
		var env = []string{"LDFLAGS=-s"}
		var torConf []string
		if runtime.GOOS == "windows" {
			env = append(env, "LIBS=-lcrypt32 -lgdi32")
		}
		torConf = []string{"sh", "./configure", "--prefix=" + pwd + "/dist",
			"--disable-gcc-hardening", "--disable-system-torrc", "--disable-asciidoc",
			"--enable-static-libevent", "--with-libevent-dir=" + pwd + "/../libevent/dist",
			"--enable-static-openssl", "--with-openssl-dir=" + pwd + "/../_openssl/dist",
			"--enable-static-zlib", "--with-zlib-dir=" + pwd + "/../zlib/dist",
			"--disable-systemd", "--disable-lzma", "--disable-seccomp",
			"--disable-html-manual", "--disable-manpage"}

		if host != "" {
			torConf = append(torConf, "--host="+host)
		}

		if runtime.GOOS == "darwin" {
			torConf = append(torConf, []string{"--disable-zstd", "--disable-libscrypt"}...)
			if host != "" {
				torConf = append(torConf, "--disable-tool-name-check")
			}
		}

		if runtime.GOOS != "darwin" {
			torConf = append(torConf, "--enable-static-tor")
		}

		if runtime.GOOS == "windows" {
			torConf = append(torConf, "--disable-zstd")
		}
		return runCmds(folder, env, [][]string{
			{"sh", "-l", "./autogen.sh"},
			torConf,
			{"make", numJobs},
			{"make", "install"},
		})
	default:
		return fmt.Errorf("unrecognized folder: %v", folder)
	}
}

func clean(folder string) (err error) {
	log.Printf("*** Cleaning %v ***", folder)
	defer log.Printf("*** Done cleaning %v ***", folder)
	switch folder {
	case "all":
		for _, subFolder := range folders {
			if err = clean(subFolder); err != nil {
				break
			}
		}
	default:
		args := []string{"clean"}
		env := []string{}
		makefile := "Makefile"
		switch folder {
		// OpenSSL needs to have the dist folder removed first
		case "_openssl":
			if err := os.RemoveAll("_openssl/dist/lib"); err != nil {
				return fmt.Errorf("unable to remove _openssl/dist/lib: %v", err)
			}
		// Zlib needs to have a prefix and needs a special windows makefile
		case "zlib":
			env = append(env, "PREFIX="+absCurrDir+"/zlib/dist")
			if runtime.GOOS == "windows" {
				makefile = "win32/Makefile.gcc"
				args = append(args, "-fwin32/Makefile.gcc")
			}
		}
		if dir, err := os.Stat(folder); err != nil || !dir.IsDir() {
			return fmt.Errorf("%v is not a directory", folder)
		} else if _, err := os.Stat(path.Join(folder, makefile)); os.IsNotExist(err) {
			log.Printf("Skipping clean, makefile not present")
			return nil
		}
		err = runCmd(folder, env, "make", args...)
	}
	return err
}

func runCmds(folder string, env []string, cmdsAndArgs [][]string) error {
	for _, cmdAndArgs := range cmdsAndArgs {
		if err := runCmd(folder, env, cmdAndArgs[0], cmdAndArgs[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func runCmd(folder string, env []string, cmd string, args ...string) error {
	log.Printf("Running in folder %v: %v %v", folder, cmd, strings.Join(args, " "))
	c := exec.Command(cmd, args...)
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	c.Dir = folder
	if verbose {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	return c.Run()
}

type libSet struct {
	dir  string
	libs []string
}

// Results in recommended linker order
func getLibSets() ([]*libSet, error) {
	// Ask Tor for their libs
	cmd := exec.Command("make", "show-libs")
	cmd.Dir = "tor"
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed 'make show-libs' in tor: %v", err)
	}
	// Load them all
	libSets := []*libSet{}
	libSetsByDir := map[string]*libSet{}
	for _, lib := range strings.Split(strings.TrimSpace(string(out)), " ") {
		dir, file := path.Split(lib)
		dir = path.Join("tor", dir)
		set := libSetsByDir[dir]
		if set == nil {
			set = &libSet{dir: dir}
			libSets = append(libSets, set)
			libSetsByDir[dir] = set
		}
		set.libs = append(set.libs, strings.TrimPrefix(strings.TrimSuffix(file, ".a"), "lib"))
	}
	// Add the rest of the known libs
	libSets = append(libSets,
		&libSet{"libevent/dist/lib", []string{"event"}},
		&libSet{"xz/dist/lib", []string{"lzma"}},
		&libSet{"zlib/dist/lib", []string{"z"}},
		&libSet{"_openssl/dist/lib", []string{"ssl", "crypto"}},
	)
	return libSets, nil
}

func showLibs() error {
	libSets, err := getLibSets()
	if err != nil {
		return err
	}
	for _, libSet := range libSets {
		fmt.Print("-L" + libSet.dir)
		for _, lib := range libSet.libs {
			fmt.Print(" -l" + lib)
		}
		fmt.Println()
	}
	return nil
}

func packageLibs() error {
	// Make both a libs.tar.gz and a libs.zip...
	// Get lib sets
	libSets, err := getLibSets()
	if err != nil {
		return err
	}
	// Create tar writer
	var tw *tar.Writer
	if tf, err := os.Create("libs.tar.gz"); err != nil {
		return err
	} else {
		defer tf.Close()
		gw := gzip.NewWriter(tf)
		defer gw.Close()
		tw = tar.NewWriter(gw)
		defer tw.Close()
	}
	tarWrite := func(path string, b []byte, i os.FileInfo) error {
		th, err := tar.FileInfoHeader(i, "")
		if err == nil {
			th.Name = path
			if err = tw.WriteHeader(th); err == nil {
				_, err = tw.Write(b)
			}
		}
		return err
	}
	// Create zip writer
	var zw *zip.Writer
	if zf, err := os.Create("libs.zip"); err != nil {
		return err
	} else {
		defer zf.Close()
		zw = zip.NewWriter(zf)
		defer zw.Close()
	}
	zipWrite := func(path string, b []byte, i os.FileInfo) error {
		zh, err := zip.FileInfoHeader(i)
		if err == nil {
			zh.Name = path
			zh.Method = zip.Deflate
			var w io.Writer
			if w, err = zw.CreateHeader(zh); err == nil {
				_, err = w.Write(b)
			}
		}
		return err
	}
	// Copy over each lib
	fileBytesAndInfo := func(name string) (b []byte, i os.FileInfo, err error) {
		f, err := os.Open(name)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()
		if i, err = f.Stat(); err == nil {
			b, err = ioutil.ReadAll(f)
		}
		return
	}
	copyFile := func(filePath string) error {
		if b, i, err := fileBytesAndInfo(filePath); err != nil {
			return err
		} else if err := tarWrite(filePath, b, i); err != nil {
			return err
		} else {
			return zipWrite(filePath, b, i)
		}
	}
	for _, libSet := range libSets {
		for _, lib := range libSet.libs {
			if err := copyFile(path.Join(libSet.dir, "lib"+lib+".a")); err != nil {
				return err
			}
		}
	}
	// Also copy over tor_api.h
	return copyFile("tor/src/feature/api/tor_api.h")
}
