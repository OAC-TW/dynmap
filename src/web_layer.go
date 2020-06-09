package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
)


// list / new / edit / del layer
func (wb *WebAPI) layer(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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

	parseColor := func(s string) (string, bool) { // only #000~#fff & #000000~#ffffff
		if len(s[1:]) < 3 {
			return "", false
		}
		if len(s[1:]) > 6 {
			return "", false
		}
		if s[0] != '#' {
			return "", false
		}
		for _, r := range s[1:] {
			if (r < 'a' || r > 'f') && (r < 'A' || r > 'F')  && (r < '0' || r > '9') {
				return "", false
			}
		}
		return s, true
	}

	parseOpacity := func(input string) (float32, error) {
		opac, err := strconv.ParseFloat(input, 32)
		if err != nil {
			return 0, err
		}
		if opac > 1.0 {
			return 1.0, nil
		}
		if opac <= 0.0 {
			return 0.0, nil
		}
		return float32(opac), nil
	}

	parseLayer := func() (*LayerGroup, string) {
		o := &LayerGroup{
			Token: r.Form.Get("token"),
			Attribution: r.Form.Get("attr"),

			Name: r.Form.Get("name"),
			Note: r.Form.Get("note"),
		}

		o.Hide = false
		if r.Form.Get("hide") == "1" {
			o.Hide = true
		}

		o.Show = false
		if r.Form.Get("show") == "1" {
			o.Show = true
		}


		o.FillColor = "#3388FF"
		fillcolor, ok := parseColor(r.Form.Get("fillcolor"))
		if ok {
			o.FillColor = fillcolor
		}

		o.Color = "#3388FF"
		color, ok := parseColor(r.Form.Get("color"))
		if ok {
			o.Color = color
		}

		o.Opacity = 0.5
		opac, err := parseOpacity(r.Form.Get("opacity"))
		if err == nil {
			o.Opacity = opac
		}

		o.UV = false
		if r.Form.Get("uv") == "1" {
			o.UV = true
		}

		return o, ""
	}


	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get layer
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			layer := wb.db.GetLayerByID(LayerID(id))
			if layer == nil {
				http.Error(w, "404 not found", http.StatusNotFound)
				return
			}

			enc := json.NewEncoder(w)
			err = enc.Encode(layer)
			if err != nil {
				// should not error, log it
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			}
			return
		}

		// get all layer
		list := wb.db.GetAllLayer()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update layer
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			layer, msg := parseLayer()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddLayer(layer)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		case "order": // order
			list := ([]LayerID)(splitParms(r.Form.Get("order")))
			if len(list) == 0 {
				writeResp(w, false, "no valid values")
				return
			}
			err := wb.db.OrderLayer(list)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		default: // update / del
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			switch act {
			case "del":
				err = wb.db.DelLayerByID(LayerID(id))

			default:
				layer, msg := parseLayer()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				layer.ID = LayerID(id)
				err = wb.db.UpdateLayer(layer)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][layer]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

