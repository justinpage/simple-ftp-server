// References:
// https://cr.yp.to/ftp.html
// https://tools.ietf.org/html/rfc959
// https://github.com/torbiak/gopl/tree/master/ex8.2
// https://github.com/kspviswa/lsgo/blob/master/ls.go
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
)

// List of FTP server return codes
const (
	AcceptedDataConnection       = "150 Accepted data connection\n"
	TypeIsNow8BitBinary          = "200 TYPE is now 8-bit binary\n"
	SystemStatus                 = "211 no-features\n"
	FileStatus                   = "213 %s\n"
	NameSystemType               = "215 UNIX Type: L8\n"
	ServiceReadyForNewUser       = "220 Service ready for new user\n"
	ServiceClosingConnection     = "221 Service closing control connection\n"
	RequestedFileActionTaken     = "226 File successfully transferred\n"
	ClosingDataConnection        = "226 Closing data connection\n"
	EnteringPassiveMode          = "227 Entering Passive Mode (%s)\n"
	UserLoggedInProceed          = "230 User logged in, proceed\n"
	RequestedFileActionCompleted = "250 OK. Current directory is %s\n"
	PathNameDeleted              = "250 Deleted %s\n"
	PathNameCreated              = "257 Created \"%s\"\n"
	CurrentWorkingDirectory      = "257 \"%s\"\n"
	UserOkayNeedPassword         = "331 User %s okay, need password\n"
	RequestedFileActionNotTaken  = "450 Requested file action not taken\n"
	RequestedActionHasFailed     = "500 Requested action has failed \"%s\"\n"
	CommandNotImplemented        = "502 Command not implemented \"%s\"\n"
	CanOnlyRetrieveRegularFiles  = "550 Can only retrieve regular files\n"
	NoSuchFileOrDirectory        = "550 No such file or directory %s\n"
	CantChangeDirectory          = "550 Not a directory %s\n"
	CantCreateDirectory          = "550 Can't create existing directory\n"
	CanOnlyDeleteRegularFiles    = "550 Can only delete regular files\n"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatalln(err)
	}

	temp, err := seedFolder()
	if err != nil {
		log.Fatalln(err)
	}

	handleClose(temp)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err) // e.g., connection aborted
			continue
		}

		s := &server{sess: conn, root: temp, path: temp}

		s.handleResponse(ServiceReadyForNewUser) // automatically accept

		go handleConn(s) // handle connections concurrently
	}
}

func handleConn(s *server) {
	defer s.sess.Close()

	cmd := bufio.NewScanner(s.sess)
	for cmd.Scan() {
		cmd := cmd.Text()
		arg := strings.Split(cmd, " ")

		if len(arg) > 1 {
			cmd = arg[0]
		}

		switch cmd {
		case "USER":
			s.handleResponse(fmt.Sprintf(UserOkayNeedPassword, arg[1]))
		case "PASS":
			s.handleResponse(UserLoggedInProceed)
		case "SYST":
			s.handleResponse(fmt.Sprintf(NameSystemType))
		case "FEAT":
			s.handleResponse(SystemStatus)
		case "QUIT":
			s.handleResponse(ServiceClosingConnection)
			return
		case "EPSV":
			s.handleResponse(fmt.Sprintf(CommandNotImplemented, cmd))
		case "PASV":
			s.handlePassive()
		case "LIST":
			s.handleList(arg)
		case "TYPE":
			s.handleResponse(TypeIsNow8BitBinary)
		case "SIZE":
			s.handleSize(arg)
		case "RETR":
			s.handleRetrieve(arg)
		case "NLST":
			s.handleNameList(arg)
		case "PWD":
			s.handlePrintWorkingDirectory()
		case "CWD":
			s.handleChangeWorkingDirectory(arg)
		case "MKD":
			s.handleMakeDirectory(arg)
		case "XMKD":
			s.handleMakeDirectory(arg)
		case "RMD":
			s.handleRemoveDirectory(arg)
		case "XRMD":
			s.handleRemoveDirectory(arg)
		case "DELE":
			s.handleDelete(arg)
		case "STOR":
			s.handleStore(arg)
		default:
			fmt.Println("cmd", cmd)
			s.handleResponse(fmt.Sprintf(CommandNotImplemented, cmd))
		}
	}

	if err := cmd.Err(); err != nil {
		log.Println(err) // something went wrong (not io.EOF)
		return
	}
}

func handleClose(path string) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.RemoveAll(path)
		os.Exit(0)
	}()
}

func seedFolder() (string, error) {
	temp, err := ioutil.TempDir("", "ftp-")
	if err != nil {
		return "", err // unable to create temporary directory
	}
	dat := []byte("hello\nftp\n")
	err = ioutil.WriteFile(temp+"/message.md", dat, 0666)
	if err != nil {
		return "", err
	}

	err = os.Mkdir(temp+"/server", 0755)
	if err != nil {
		return "", err
	}

	dat, err = ioutil.ReadFile("./main.go")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(temp+"/server/main.go", dat, 0666)
	if err != nil {
		return "", err
	}

	err = os.Chdir(temp) // start each connection inside temp dir
	if err != nil {
		return "", err
	}

	return temp, nil
}

type server struct {
	sess net.Conn
	pasv net.Listener
	root string
	path string
}

func (s *server) handleResponse(msg string) {
	_, err := io.WriteString(s.sess, msg)
	if err != nil {
		log.Println(err)
		return // e.g., client disconnected
	}
}

func (s *server) handlePassive() {
	var err error
	s.pasv, err = net.Listen("tcp", "") // port automatically chosen

	_, p, err := net.SplitHostPort(s.pasv.Addr().String())
	h, _, err := net.SplitHostPort(s.sess.LocalAddr().String())

	addr, err := net.ResolveIPAddr("", h)
	port, err := strconv.ParseInt(p, 10, 64)

	ip := addr.IP.To4()

	location := fmt.Sprintf(
		"%d,%d,%d,%d,%d,%d", ip[0], ip[1], ip[2], ip[3], port/256, port%256,
	)

	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "PASV"))
		return
	}

	s.handleResponse(fmt.Sprintf(EnteringPassiveMode, location))
}

func (s *server) handleList(arg []string) {
	conn, err := s.pasv.Accept()
	if err != nil {
		log.Println(err) // e.g., connection aborted
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "LIST"))
		return
	}

	defer conn.Close()

	s.handleResponse(AcceptedDataConnection)

	tw := new(tabwriter.Writer).Init(conn, 0, 8, 2, ' ', 0)

	list := func(file os.FileInfo) {
		const format = "%s\t%3v %s\t%s\t%12v %s %s\r\n"

		mode := file.Mode().String()
		link := file.Sys().(*syscall.Stat_t).Nlink

		uid := strconv.Itoa(int(file.Sys().(*syscall.Stat_t).Uid))
		owner, _ := user.LookupId(uid)
		username := owner.Username

		gid := strconv.Itoa(int(file.Sys().(*syscall.Stat_t).Gid))
		group, _ := user.LookupGroupId(gid)
		groupname := group.Name

		size := file.Size()
		time := file.ModTime().Format("Jan  2 15:04")
		name := file.Name()

		fmt.Fprintf(tw, format, mode, link, username,
			groupname, size, time, name,
		)

		tw.Flush()
	}

	switch a := len(arg); a {
	// list current working directory
	case 1:
		files, err := ioutil.ReadDir(s.path)
		if err != nil {
			log.Println(err)
			s.handleResponse(RequestedFileActionNotTaken)
			return
		}

		for _, file := range files {
			list(file)
		}
	// list specific file or directory content
	case 2:
		dir := filepath.Clean(arg[1])
		path, _ := filepath.Abs(filepath.Join(s.path, dir))

		// Prevent listing a directory above root
		if !strings.HasPrefix(path, s.root) {
			dir := filepath.Clean("/" + dir)
			path, _ = filepath.Abs(filepath.Join(s.root, dir))
		}

		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			log.Println(err)
			s.handleResponse(fmt.Sprintf(NoSuchFileOrDirectory, dir))
			return
		}
		if err != nil {
			log.Println(err)
			s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "LIST"))
			return
		}
		if !info.IsDir() {
			list(info)
			break
		}

		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Println(err)
			s.handleResponse(RequestedFileActionNotTaken)
			return
		}

		for _, file := range files {
			list(file)
		}
	}

	s.handleResponse(ClosingDataConnection)
}

func (s *server) handleSize(arg []string) {
	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "SIZE"))
		return
	}

	info, err := file.Stat()
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "SIZE"))
		return
	}

	s.handleResponse(fmt.Sprintf(FileStatus, info.Size()))
}

func (s *server) handleRetrieve(arg []string) {
	conn, err := s.pasv.Accept()
	if err != nil {
		log.Println(err) // e.g., connection aborted
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "RETR"))
		return
	}

	defer conn.Close()

	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	// Prevent retrieving a remote-file above root
	if !strings.HasPrefix(path, s.root) {
		dir := filepath.Clean("/" + dir)
		path, _ = filepath.Abs(filepath.Join(s.root, dir))
	}

	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(RequestedFileActionNotTaken)
		return
	}

	info, err := file.Stat()
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "RETR"))
		return
	}
	if info.IsDir() {
		s.handleResponse(CanOnlyRetrieveRegularFiles)
		return
	}

	s.handleResponse(AcceptedDataConnection)

	_, err = io.Copy(conn, file)
	if err != nil {
		log.Println(err)
		s.handleResponse(RequestedFileActionNotTaken)
		return
	}

	s.handleResponse(RequestedFileActionTaken)
}

func (s *server) handleNameList(arg []string) {
	conn, err := s.pasv.Accept()
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "NLST"))
		return
	}

	defer conn.Close()

	s.handleResponse(AcceptedDataConnection)

	path := s.path
	if len(arg) > 1 {
		// Support sub-directory listing when available
		path, _ = filepath.Abs(filepath.Join(s.path, arg[1]))
	}

	// Prevent listing a directory above root
	if !strings.HasPrefix(path, s.root) {
		dir := filepath.Clean("/" + arg[1])
		path, _ = filepath.Abs(filepath.Join(s.root, dir))
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(RequestedFileActionNotTaken)
		return
	}

	for _, file := range files {
		fmt.Fprintf(conn, "%s\r\n", file.Name())
	}

	s.handleResponse(ClosingDataConnection)
}

func (s *server) handlePrintWorkingDirectory() {
	// Print base directory instead of full path
	// (e.g. /dir instead of /root/dir)
	dir := strings.Split(s.path, s.root)[1]
	if dir != "" {
		s.handleResponse(fmt.Sprintf(CurrentWorkingDirectory, dir))
		return
	}

	s.handleResponse(fmt.Sprintf(CurrentWorkingDirectory, "/"))
}

func (s *server) handleChangeWorkingDirectory(arg []string) {
	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	// Prevent changing to a directory above root
	if !strings.HasPrefix(path, s.root) {
		s.path = s.root
		s.handleResponse(fmt.Sprintf(RequestedFileActionCompleted, "/"))
		return
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		s.handleResponse(fmt.Sprintf(NoSuchFileOrDirectory, dir))
		return
	}
	if !info.IsDir() {
		s.handleResponse(fmt.Sprintf(CantChangeDirectory, dir))
		return
	}
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "CWD"))
		return
	}

	s.path = path
	if d := strings.Split(s.path, s.root)[1]; d != "" {
		s.handleResponse(fmt.Sprintf(RequestedFileActionCompleted, d))
		return
	}

	s.handleResponse(fmt.Sprintf(RequestedFileActionCompleted, "/"))
}

func (s *server) handleMakeDirectory(arg []string) {
	if len(arg) != 2 {
		s.handleResponse("usage: mkdir directory-name\n")
	}

	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	// Check if the parent directory exists before creating children
	info, err := os.Stat(filepath.Dir(path))
	if os.IsNotExist(err) {
		s.handleResponse(fmt.Sprintf(NoSuchFileOrDirectory, dir))
		return
	}
	if !info.IsDir() {
		s.handleResponse(fmt.Sprintf(CantChangeDirectory, dir))
		return
	}
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "MKD"))
		return
	}

	// Prevent creating a directory above root
	if !strings.HasPrefix(path, s.root) {
		dir := filepath.Clean("/" + dir)
		path, _ := filepath.Abs(filepath.Join(s.root, dir))

		err := os.Mkdir(path, 0755)
		if os.IsExist(err) {
			s.handleResponse(fmt.Sprintf(CantCreateDirectory))
			return
		}
		if err != nil {
			log.Println(err)
			s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "MKD"))
			return
		}

		s.handleResponse(fmt.Sprintf(PathNameCreated, dir))
		return
	}

	err = os.Mkdir(path, 0755)
	if os.IsExist(err) {
		s.handleResponse(fmt.Sprintf(CantCreateDirectory))
		return
	}
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "MKD"))
		return
	}

	s.handleResponse(fmt.Sprintf(PathNameCreated, dir))
}

func (s *server) handleRemoveDirectory(arg []string) {
	if len(arg) != 2 {
		s.handleResponse("usage: rm directory-name\n")
	}

	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	// Prevent deleting a directory above root
	if !strings.HasPrefix(path, s.root) {
		dir := filepath.Clean("/" + dir)
		path, _ = filepath.Abs(filepath.Join(s.root, dir))
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		s.handleResponse(fmt.Sprintf(NoSuchFileOrDirectory, dir))
		return
	}
	if !info.IsDir() {
		s.handleResponse(fmt.Sprintf(CantChangeDirectory, dir))
		return
	}
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "RMD"))
		return
	}

	err = os.RemoveAll(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "RMD"))
		return
	}

	s.handleResponse(fmt.Sprintf(PathNameDeleted, dir))
}

func (s *server) handleDelete(arg []string) {
	if len(arg) != 2 {
		s.handleResponse("usage: delete remote-file\n")
	}

	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	// Prevent deleting a remote-file above root
	if !strings.HasPrefix(path, s.root) {
		dir := filepath.Clean("/" + dir)
		path, _ = filepath.Abs(filepath.Join(s.root, dir))
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		s.handleResponse(fmt.Sprintf(NoSuchFileOrDirectory, dir))
		return
	}
	if info.IsDir() {
		s.handleResponse(CanOnlyDeleteRegularFiles)
		return
	}
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "DELE"))
		return
	}

	err = os.Remove(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "DELE"))
		return
	}

	s.handleResponse(fmt.Sprintf(PathNameDeleted, dir))
}

func (s *server) handleStore(arg []string) {
	conn, err := s.pasv.Accept()
	if err != nil {
		log.Println(err) // e.g., connection aborted
		s.handleResponse(fmt.Sprintf(RequestedActionHasFailed, "STOR"))
		return
	}

	defer conn.Close()

	s.handleResponse(AcceptedDataConnection)

	dir := filepath.Clean(arg[1])
	path, _ := filepath.Abs(filepath.Join(s.path, dir))

	file, err := os.Create(path)
	if err != nil {
		log.Println(err)
		s.handleResponse(RequestedFileActionNotTaken)
		return
	}

	_, err = io.Copy(file, conn)
	if err != nil {
		log.Println(err)
		s.handleResponse(RequestedFileActionNotTaken)
		return
	}

	s.handleResponse(RequestedFileActionTaken)
}
