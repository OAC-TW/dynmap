# 簡易Web圖台
透過[Leaflet.js](https://leafletjs.com/)及各種plugin, 由client端繪製並顯示各種圖資;

搭配簡易後台方便更新圖資、連結等等的額外資訊,

達到最小化後端系統需求及相依性


## 當初要求
* 不使用ArcGIS Server
* 要有後台系統可以動態更新幾乎所有資料、設定
* 非資訊人員也能像打Word一樣輕鬆編輯
* 資料量不大(< 100MB), 且資料、設定更新頻率不高
* **架設環境很有可能是Android平板 + 4G網路**

### 因此採用以下設計
* 由client端繪製圖資
* 用[Quill.js](https://quilljs.com/)當作WYSIWYG editor, 後端直接保存Delta資料, 吐回前端後再由Quill.js驗證&轉成安全的HTML code
* 儘可能減少檔案IO、網路頻寬使用、CPU使用
	* 檔案資源儘可能由service worker cache起來
	* 幾乎所有資料放至於RAM
	* 儘可能減少render次數、量, 將render結果cache起來
* 伺服器&資料儲存不要有其他相依性
	* 不使用額外服務(no DB Server), 直接內建簡易db功能
	* 可編譯成static link, 減少相依性


----

## 程式碼架構
* `/src/` 伺服器原始碼
	* `api.go` 定義api界面
	* `logger.go` log函數, 自帶簡易rotate
	* `utility.go` 工具函數
	* `db_json.go` 實做儲存界面
	* `web_*.go` web相關的handler、函數
	* 剩下的`*.go` 相關物件的序列化、反序列化、基本操作(新增、修改、刪除)
* `/www/` 後台、相依的js library、css存放位置
	* `/www/admin/` 後台SPA code
	* `/www/res/` 相依的js library、css存放位置, 應不常改動, 可長期cache
* `index.tmpl` 圖台(首頁)模板
* `sw.js.tmpl` service worker模板
* `main.go` 設定檔、web server初始化

## 執行目錄架構
* `/upload/` 預設上傳檔案存放位置
* `/log/` 預設log檔存放位置
* `/www/` 後台、相依的js library、css存放位置
* `index.tmpl` 圖台(首頁)模板
* `sw.js.tmpl` service worker模板
* `xxx.exe` / `xxx.elf` 伺服器執行檔

----

## 編譯&執行

```bash
git clone --depth 1 "https://github.com/cs8425/dynmap.git"
cd dynmap/
go build . # build
./main # start server
# open http://127.0.0.1:4040/
```

### 第一個管理帳號
1. 用參數`-ssusr`啟動伺服器, `test`為臨時登入用的帳號名
```bash
./main -ssusr test  # start server with temporary user 'test'
```

2. 會隨機產生密碼出現類似以下訊息:
```
2020/06/09 13:38:08 [warn]enable temporary super user: test NUrcHr#hB+
```

3. 開啟http://127.0.0.1:4040/admin/

用`test` / `NUrcHr#hB+`登入後台設定帳號

下次啟動時不必再加上`-ssusr`參數

----

### 動態資料更新

1. 新增任一動態資源
2. 取得兩個token, `存取代碼`(token)為一般圖層引用之參數, `更新代碼`(AuthToken)為爬蟲等外部動態更新用
	* 例如: `存取代碼`(token) = `xeNWkSuV7vDTptKLMKvQ`, `更新代碼`(AuthToken) = `89HuRzqCRlRGIrhSifYN`
	* 測試資料更新、取得
```bash
curl -v -X POST --form "file=@README.md" "http://127.0.0.1:4040/api/push/89HuRzqCRlRGIrhSifYN" # send README.md
curl -v "http://127.0.0.1:4040/hook/xeNWkSuV7vDTptKLMKvQ" # get data back
```
3. 設定爬蟲等外部程式使用`更新代碼`(AuthToken)推送新資料

