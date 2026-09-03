package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/local/sftpweb/internal/auth"
	"github.com/local/sftpweb/internal/sftpconn"
)

// browseRoot is the boundary for path sanitisation. The remote SFTP server remains
// the authority on what the authenticated user may actually reach.
const browseRoot = "/"

type connectRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type connectionInfo struct {
	Connected bool   `json:"connected"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Username  string `json:"username,omitempty"`
	Home      string `json:"home,omitempty"`
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())

	var req connectRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "malformed JSON body")
		return
	}
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	if req.Port == 0 {
		req.Port = 22
	}
	if req.Host == "" || req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "host, username and password are required")
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		writeErr(w, http.StatusBadRequest, "bad_request", "port must be between 1 and 65535")
		return
	}

	conn, err := s.pool.Connect(sess.ID, sftpconn.Credentials{
		Host: req.Host, Port: req.Port, User: req.Username, Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, sftpconn.ErrHostDenied) || errors.Is(err, sftpconn.ErrPoolFull) {
			writeSFTPErr(w, err)
			return
		}
		writeErr(w, http.StatusBadGateway, "connect_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connectionInfo{Connected: true, Host: conn.Host, Port: conn.Port, Username: conn.User, Home: conn.Home})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	s.pool.Disconnect(sess.ID)
	writeJSON(w, http.StatusOK, connectionInfo{Connected: false})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	conn, err := s.pool.Get(sess.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, connectionInfo{Connected: false})
		return
	}
	writeJSON(w, http.StatusOK, connectionInfo{Connected: true, Host: conn.Host, Port: conn.Port, Username: conn.User, Home: conn.Home})
}

type entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"isDir"`
	IsLink  bool   `json:"isLink"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
}

type listResponse struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent"`
	Entries []entry `json:"entries"`
}

// conn resolves the caller's SFTP connection and the requested path in one step.
func (s *Server) conn(r *http.Request, rawPath string) (*sftpconn.Conn, string, error) {
	sess, ok := auth.FromContext(r.Context())
	if !ok {
		return nil, "", sftpconn.ErrNotConnected
	}
	c, err := s.pool.Get(sess.ID)
	if err != nil {
		return nil, "", err
	}
	if rawPath == "" {
		rawPath = c.Home
	}
	p, err := CleanPath(browseRoot, rawPath)
	if err != nil {
		return nil, "", err
	}
	return c, p, nil
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	c, dir, err := s.conn(r, r.URL.Query().Get("path"))
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	c.Lock()
	infos, err := c.Client.ReadDir(dir)
	c.Unlock()
	if err != nil {
		writeSFTPErr(w, err)
		return
	}

	entries := make([]entry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, entry{
			Name:    fi.Name(),
			Path:    path.Join(dir, fi.Name()),
			Size:    fi.Size(),
			IsDir:   fi.IsDir(),
			IsLink:  fi.Mode()&fs.ModeSymlink != 0,
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime().UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, listResponse{Path: dir, Parent: path.Dir(dir), Entries: entries})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	c, p, err := s.conn(r, r.URL.Query().Get("path"))
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	c.Lock()
	defer c.Unlock()

	fi, err := c.Client.Stat(p)
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	if fi.IsDir() {
		writeErr(w, http.StatusBadRequest, "is_directory", "cannot download a directory")
		return
	}
	f, err := c.Client.Open(p)
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	defer f.Close()

	name := path.Base(p)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, f); err != nil {
		// Headers are already sent; the client sees a truncated body.
		return
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	c, dirPath, err := s.conn(r, dir)
	if err != nil {
		writeSFTPErr(w, err)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "expected multipart/form-data")
		return
	}

	uploaded := []string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "malformed multipart body")
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		// Reject any directory component a browser may include in the filename.
		name := path.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		if name == "." || name == "/" || name == ".." {
			_ = part.Close()
			writeErr(w, http.StatusBadRequest, "bad_path", "invalid file name")
			return
		}
		dest := path.Join(dirPath, name)

		c.Lock()
		dst, err := c.Client.Create(dest)
		if err != nil {
			c.Unlock()
			_ = part.Close()
			writeSFTPErr(w, err)
			return
		}
		n, copyErr := io.Copy(dst, io.LimitReader(part, s.maxUpload+1))
		closeErr := dst.Close()
		c.Unlock()
		_ = part.Close()

		if copyErr != nil {
			writeSFTPErr(w, copyErr)
			return
		}
		if n > s.maxUpload {
			c.Lock()
			_ = c.Client.Remove(dest)
			c.Unlock()
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("file exceeds the %d byte limit", s.maxUpload))
			return
		}
		if closeErr != nil {
			writeSFTPErr(w, closeErr)
			return
		}
		uploaded = append(uploaded, dest)
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploaded": uploaded})
}

type pathRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "malformed JSON body")
		return
	}
	c, p, err := s.conn(r, req.Path)
	if err != nil || req.Path == "" {
		writeSFTPErr(w, orBadPath(err))
		return
	}
	c.Lock()
	if _, statErr := c.Client.Stat(p); statErr == nil {
		c.Unlock()
		writeErr(w, http.StatusConflict, "exists", "something with that name already exists")
		return
	}
	err = c.Client.Mkdir(p)
	c.Unlock()
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": p})
}

type renameRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "malformed JSON body")
		return
	}
	c, from, err := s.conn(r, req.From)
	if err != nil || req.From == "" || req.To == "" {
		writeSFTPErr(w, orBadPath(err))
		return
	}
	to, err := CleanPath(browseRoot, req.To)
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	c.Lock()
	if _, statErr := c.Client.Stat(to); statErr == nil {
		c.Unlock()
		writeErr(w, http.StatusConflict, "exists", "something with that name already exists")
		return
	}
	err = c.Client.Rename(from, to)
	c.Unlock()
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": to})
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	c, p, err := s.conn(r, r.URL.Query().Get("path"))
	if err != nil || r.URL.Query().Get("path") == "" {
		writeSFTPErr(w, orBadPath(err))
		return
	}
	if p == "/" {
		writeErr(w, http.StatusBadRequest, "bad_path", "refusing to remove the root directory")
		return
	}
	c.Lock()
	fi, statErr := c.Client.Stat(p)
	if statErr == nil && fi.IsDir() {
		err = c.Client.RemoveDirectory(p)
	} else {
		err = c.Client.Remove(p)
	}
	c.Unlock()
	if statErr != nil {
		writeSFTPErr(w, statErr)
		return
	}
	if err != nil {
		writeSFTPErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": p})
}

func orBadPath(err error) error {
	if err == nil {
		return ErrBadPath
	}
	return err
}
