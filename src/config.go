package webmap

import (
	"time"
)

type TmplIndex struct {
	SiteTitle string `json:"title,omitempty"`
	Logo string `json:"logo,omitempty"`

	Lang string `json:"lang,omitempty"` // zh-Hant
	Manifest string `json:"manifest,omitempty"` // json string

	// TODO: better way to custom head tag
	HtmlHead string `json:"head,omitempty"` // raw html string

//	PWAShortName string `json:"pwa_short_name,omitempty"`
//	PWAName string `json:"pwa_name,omitempty"`
//	PWADesc string `json:"description,omitempty"`
//	PWAStartUrl string `json:"start_url,omitempty"`
//	PWAOrientation string `json:"orientation,omitempty"`
//	PWAIcon string `json:"icon,omitempty"`

	CountStats bool `json:"cstats,omitempty"`
	ShowStats bool `json:"stats,omitempty"`
	ShowLink bool `json:"link,omitempty"`

	ShowUserLoad bool `json:"load,omitempty"`
	LoadLimit int64 `json:"loadfs,omitempty"`

	//Tabs []*TabData `json:"tabs,omitempty"`

	// for sw/Etag cache control
	VersionD string `json:"verD,omitempty"` // Layer's res token change
	VersionC string `json:"verC,omitempty"` // config of Layer, Map, Link, Anno
	VersionA string `json:"verA,omitempty"` // base html, base css, SiteTitle, Watermark
}

func (s *TmplIndex) Clone() *TmplIndex {
	s2 := *s
	return &s2
}

func genVersion() string {
	return formatTimestamp(time.Now()) + "-" + genRng8()
}


