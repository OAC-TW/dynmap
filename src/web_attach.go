package webmap

import (
	"encoding/json"
	"os"
	"mime/multipart"
	"net/http"
	"path/filepath"
)

var (
	UploadFileSizeLimit = int64(100 * 1024 * 1024) // Bytes (100 MB)
	UploadFileDir = "./upload/"
)

func (wb *WebAPI) download(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	fileToken := filepath.Base(r.URL.Path) // TODO: check 'asd?foo=bar'
	attach := wb.db.GetAttachByToken(fileToken)
	if attach == nil {
		goto ERR404
	}

	if attach.Hide { // deleted
		if sd == nil {
			goto ERR404
		}
		uid, ok := sd.Get("acc")
		if !ok {
			goto ERR404
		}
		u := wb.db.GetUserByUID(uid.(UserID))
		if u == nil {
			goto ERR404
		}
		if u.Freeze {
			goto ERR404
		}
		goto DL
	}

DL:
	attach.ServeContent(w, r, UploadFileDir)
	return

ERR404:
	http.Error(w, "404 not found", http.StatusNotFound)
	return
}

func (wb *WebAPI) attach(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	uid, ok := sd.Get("acc")
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	u := wb.db.GetUserByUID(uid.(UserID))
	if u == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if u.Freeze {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	switch r.Method {
	case "GET": // get all attach info
		list := wb.db.ListAttach() // TODO: limit & page
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}


	case "POST": // upload / delete attach
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		//act := getKey(r.URL.Path) // '/api/attach/upload' or '/api/attach/del'
		token, act := getParm(r.URL.Path, base)
		switch token {
		case "": // upload

			// TODO: session timeout when uploading?
			r.ParseMultipartForm(UploadFileSizeLimit)

			fhs := r.MultipartForm.File["attach"]
			for _, fh := range fhs {
				file, err := fh.Open()
				if err != nil {
					Vln(3, "[web][upload]parse file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
					continue
				}
				attach, erroeText, errCode := saveFile(file, fh, r)
				if erroeText != "" && errCode != 0 {
					http.Error(w, erroeText, errCode)
					return
				}
				if attach != nil {
					attach.UploadUID = u.ID
					_, err = wb.db.AddAttach(attach)
					if err != nil {
						Vln(3, "[web][upload]api call error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}
				}
			}

			writeResp(w, true, "")


		default:
			switch act {
			case "del":
				attach := wb.db.GetAttachByToken(token)
				if attach == nil {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}
				//if u.ID != attach.UploadUID {
				//	http.Error(w, "Forbidden, no permission", http.StatusForbidden)
				//	return
				//}
				wb.db.DelAttachByAID(attach.ID)
				err := attach.DelFromFS(UploadFileDir)
				if err != nil {
					Vln(3, "[web][attach]remove error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				writeResp(w, true, "")
				return
			case "hide":
				attach := wb.db.GetAttachByToken(token)
				if attach == nil {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}
				attach.Hide = true

				err := wb.db.UpdateAttach(attach)
				if err != nil {
					Vln(3, "[web][attach]set hide error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				writeResp(w, true, "")
				return
			case "unhide":
				attach := wb.db.GetAttachByToken(token)
				if attach == nil {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}
				attach.Hide = false

				err := wb.db.UpdateAttach(attach)
				if err != nil {
					Vln(3, "[web][attach]set unhide error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				writeResp(w, true, "")
				return
			case "info":
				attach := wb.db.GetAttachByToken(token)
				if attach == nil || attach.Hide {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}

				// remove not necessary info
				attach = attach.Clone()
				attach.SaveName = ""

				enc := json.NewEncoder(w)
				err = enc.Encode(attach)
				if err != nil {
					// should not error, log it
					Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				}
				return
			default:
			}
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
	}
}

func saveFile(file multipart.File, handler *multipart.FileHeader, r *http.Request) (attach *Attachment, erroeText string, errCode int) {
	defer file.Close()

	// TODO: check meta
	// check file magic
	if isExeFile(file) {
		Vln(3, "[web][upload]try upload executable file!", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), handler.Filename, handler.Size, handler.Header)
		return nil, "file type not allow", http.StatusForbidden
	}
	attach = NewAttachment(handler.Filename, handler.Size)
	//attach.UploadUID = u.ID

	saveFp := filepath.Join(UploadFileDir, attach.SaveName)
	f, err := os.OpenFile(saveFp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		Vln(3, "[web][upload]open save file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		return nil, "Internal server error", http.StatusInternalServerError
	}
	defer f.Close()

	hash, err := cpAndHashFd(f, file)
	if err != nil {
		Vln(3, "[web][upload]save and hash file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		return nil, "Internal server error", http.StatusInternalServerError
	}
	attach.Checksum = hash
	return
}


