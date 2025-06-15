package main

import "time"

type TaggedImages []TaggedImage

type TaggedImage struct {
	ImageID    string    `json:"imageId"`
	URL        string    `json:"url"`
	URLBig     string    `json:"url_big"`
	Dim        []int     `json:"dim"`
	DimBig     []int     `json:"dim_big"`
	Thumbnail  Thumbnail `json:"thumbnail"`
	Big        Big       `json:"big"`
	CreatedAt  time.Time `json:"createdAt"`
	Prefix     string    `json:"prefix"`
	Key        string    `json:"key"`
	Height     int       `json:"height"`
	Width      int       `json:"width"`
	Expiration time.Time `json:"expiration"`
	Tags       []Tags    `json:"tags"`
	Likes      []Likes   `json:"likes"`
	Liked      bool      `json:"liked"`
}
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type Big struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type Tags struct {
	Tag     string `json:"tag"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	ChildID string `json:"childId"`
}
type Likes struct {
	Name         string `json:"name"`
	LoginID      string `json:"loginId"`
	Subtitle     string `json:"subtitle"`
	ProfileImage string `json:"profileImage"`
	Reaction     string `json:"reaction"`
}
