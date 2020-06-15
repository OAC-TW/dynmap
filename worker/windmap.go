package main

/*
* 中央氣象局 風場預報顯示圖
* https://app.cwb.gov.tw/web/cwbwifi/
* 將網站上的2020MMDDHH_UV_SFC.json抓下來
* https://app.cwb.gov.tw/web/cwbwifi/data/weather/2020061500_UV_SFC.json
*/

import (
	"flag"
//	"log"
	"time"
	"fmt"

	"io"
	"os"
//	"strings"
//	"strconv"
	//"errors"

	"encoding/json"
//	"encoding/xml"
	"math"

	"bytes"
	"crypto/tls"
	"mime/multipart"
	"net"
	"net/http"
	"io/ioutil"
)

var (
	inFile = flag.String("i", "2020061500_UV_SFC.json", "input file")
	outFile = flag.String("o", "2020061500_UV_SFC.2.json", "output file")

	url = flag.String("u", "https://app.cwb.gov.tw/web/cwbwifi/data/weather/2020061500_UV_SFC.json", "url")
	UA = flag.String("ua", "OAC bot", "User-Agent")

	hookUrl = flag.String("hook", "http://127.0.0.1:8080/api/push/2E9UTT1kI_DqOeoiugXj", "web hook URL")
)

func main() {
	flag.Parse()

	if *inFile != "" && *outFile != "" {
		transFile(*inFile, *outFile)
		return
	}

	fd, err := getUrlFd(*url)
	if err != nil {
		fmt.Println("[get]err", *url, err)
		return
	}
	defer fd.Close()

	grid, err := parseFile(fd)
	if err != nil {
		fmt.Println("[parse]err", err)
		return
	}
	fmt.Println("[grid]", grid.Nx, grid.Ny)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err = enc.Encode(grid)
	if err != nil {
		fmt.Println("[json]err", err)
	}
	fmt.Println("[json]ok")

	postUrl(*hookUrl, *outFile, &buf)
	fmt.Println("[post]", *hookUrl)
}

func transFile(inFp string, outFp string) {
	fd, err := os.OpenFile(inFp, os.O_CREATE|os.O_RDONLY, 0400)
	if err != nil {
		fmt.Println("[open]err", *inFile, err)
		return
	}
	defer fd.Close()

	grid, err := parseFile(fd)
	if err != nil {
		fmt.Println("[parse]err", err)
		return
	}
	fmt.Println("[grid]", grid.Nx, grid.Ny)

	of, err := os.OpenFile(outFp, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("[open]err", *outFile, err)
		return
	}
	defer of.Close()

	enc := json.NewEncoder(of)
	err = enc.Encode(grid)
	if err != nil {
		fmt.Println("[json]err", err)
	}
}

func getUrl(url string) ([]byte, error) {
	resBody, err := getUrlFd(url)
	if err != nil {
		return nil, err
	}
	defer resBody.Close()

	data, err := ioutil.ReadAll(resBody)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getUrlFd(url string) (io.ReadCloser, error) {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var netClient = &http.Client{
		Timeout: time.Second * 60,
		Transport: netTransport,
	}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Connection", "close")
	req.Header.Set("User-Agent", *UA)
	req.Header.Set("authority", "app.cwb.gov.tw")
	req.Header.Set("referer", "https://app.cwb.gov.tw/web/cwbwifi/")
	req.Close = true
	res, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

func postUrl(url string, fileName string, data io.Reader) ([]byte, error) {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	var netClient = &http.Client{
		Timeout: time.Second * 60,
		Transport: netTransport,
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(fw, data)
	if err != nil {
		return nil, err
	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	req, err := http.NewRequest("POST", url, nil)
	req.Header.Set("Connection", "close")
	req.Header.Set("User-Agent", *UA)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Close = true
	//req.Body = ioutil.NopCloser(data)
	req.Body = ioutil.NopCloser(&b)
	res, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	ret, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return ret, nil
}


type VectorGrid struct {
	// 原點 經度, 緯度
	Lo1 float32 `json:"lo1"`
	La1 float32 `json:"la1"`

	// 終點 經度, 緯度
	Lo2 float32 `json:"lo2"`
	La2 float32 `json:"la2"`

	Nx int `json:"nx"` // 經度格數
	Ny int `json:"ny"` // 緯度格數

	Time string `json:"time"` // just copy now
	Desc string `json:"Description"`  // just copy

	Data map[string][]jsonFloat `json:"d"`
}

type jsonFloat float32
func (value jsonFloat) MarshalJSON() ([]byte, error) {
	if math.IsNaN(float64(value)) {
		return []byte("\"\""), nil
	}
	return []byte(fmt.Sprintf("%v", value)), nil
}

func NewVectorGrid() *VectorGrid {
	vg := &VectorGrid{}
	vg.Data = make(map[string][]jsonFloat, 2)
	return vg
}


type Grib2Header struct {
	// 原點 經度, 緯度
	Lo1 float32 `json:"lo1"`
	La1 float32 `json:"la1"`

	// 終點 經度, 緯度
	Lo2 float32 `json:"lo2"`
	La2 float32 `json:"la2"`

	Nx int `json:"nx"` // 經度格數
	Ny int `json:"ny"` // 緯度格數

	Dx float32 `json:"dx"` // 經度間隔
	Dy float32 `json:"dy"` // 緯度間隔

	Time string `json:"refTime"` // just copy now

	ParameterCategory uint8 `json:"parameterCategory"`
	ParameterNumber uint8 `json:"parameterNumber"`
	ScanMode uint8 `json:"scanMode"`

	SurfacelType uint8 `json:"surfacelType"`
	SurfacelValue float32 `json:"surfacelValue"`
	ForecastTime uint8 `json:"forecastTime"`
}

type Grib2 struct {
	Header *Grib2Header `json:"header"`
	Data []jsonFloat `json:"data"`
}

func parseFile(r io.Reader) (*VectorGrid, error) {
	grid := NewVectorGrid()

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Println("[json][read]err", err)
		return nil, err
	}

	fmt.Println("[json]", len(buf))

	grb := make([]*Grib2, 0, 2)
	err = json.Unmarshal(buf, &grb)
	if err != nil {
		fmt.Println("[json][parse]err", err)
		return nil, err
	}
	fmt.Println("[grib]", len(grb))

	var x, y *Grib2
	for _, rec := range grb {
		switch rec.Header.ParameterNumber {
		case 2:
			x = rec
		case 3:
			y = rec
		}
	}

	grid.Lo1 = x.Header.Lo1
	grid.La1 = x.Header.La1
	grid.Lo2 = x.Header.Lo2
	grid.La2 = x.Header.La2
	grid.Nx = x.Header.Nx
	grid.Ny = x.Header.Ny
	grid.Time = x.Header.Time
	grid.Data["X"] = x.Data
	grid.Data["Y"] = y.Data

	fmt.Println("[grid]", grid.Nx, grid.Ny, len(grid.Data["X"]), len(grid.Data["Y"]))

	return grid, nil
}


