package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const BASE_URL = "http://www.mzitu.cc"

type Ablum struct {
	Title  string   `json:"title"`
	Href   string   `json:"href"`
	Id     int      `json:"id"`
	Cover  string   `json:"cover"`
	Images []string `json:"images"`
}

func (ablum *Ablum) FillAndDownloadImages(group *sync.WaitGroup) []string {
	defer group.Done()

	var url string = ablum.Href
	fmt.Println("download cover image:", ablum.Cover)
	coverLocalFile := fmt.Sprintf("imgs/%d_%d_cover.jpg", ablum.Id, time.Now().UnixMilli())
	err := DownloadFile(coverLocalFile, ablum.Cover)
	var noCover bool = false
	if err != nil {
		fmt.Println("download cover image error")
		os.Remove(coverLocalFile)
		noCover = true
	}

	ablum.Cover = coverLocalFile

	imageList := []string{}
	visitedUrl := make(map[string]bool)
	currentUrl := url
	visitedUrl[currentUrl] = true
	for {
		fmt.Println("request url", currentUrl)
		resp, err := http.Get(currentUrl)
		if err != nil || resp.StatusCode != 200 {
			resp.Body.Close()
			fmt.Println("request error quit.")
			return imageList
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return imageList
		}

		aTag := doc.Find(".main-image a")
		// fmt.Println(aTag.Html())
		href, exist := aTag.Attr("href")
		if !exist {
			resp.Body.Close()
			break
		}
		fmt.Println(href)
		imageUrl, srcExist := aTag.Find("img").Attr("src")
		if srcExist && imageUrl != "" {
			curTime := time.Now()
			imageLocalFile := fmt.Sprintf("imgs/%d_%d.jpg", ablum.Id, curTime.UnixMilli())
			err = DownloadFile(imageLocalFile, imageUrl)
			if err == nil {
				imageList = append(imageList, imageLocalFile)
			}
		}

		currentUrl = BASE_URL + href
		if visitedUrl[currentUrl] {
			break
		}

		visitedUrl[currentUrl] = true
		resp.Body.Close()
	} //end for each

	ablum.Images = imageList
	if noCover && len(ablum.Images) > 0 {
		ablum.Cover = ablum.Images[0]
	}
	return imageList
}

func fetchAblums(url string) []Ablum {
	fmt.Println("fetchAblums")

	var ablumList []Ablum = []Ablum{}

	resp, err := http.Get(url)
	if err != nil {
		return ablumList
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ablumList
	}
	doc.Find(".postlist li").Each(func(i int, s *goquery.Selection) {
		aTag := s.Find("a").First()
		href, _ := aTag.Attr("href")

		fmt.Println("href", href)

		imgTag := s.Find("a img")
		cover, _ := imgTag.Attr("data-original")
		title, _ := imgTag.Attr("alt")
		fmt.Println("cover", cover)
		fmt.Println("title", title)
		id := GetAidFromHref(href)
		fmt.Println("id", id)

		var ablum Ablum = Ablum{
			Title: title,
			Href:  BASE_URL + href,
			Cover: cover,
			Id:    id,
		}

		ablumList = append(ablumList, ablum)
		// fetchImages(ablum.Href, &ablum)
	})
	return ablumList
}

func GetAidFromHref(href string) int {
	if href == "" {
		return -1
	}

	var preIdx int = strings.LastIndex(href, "/")
	var endIdx int = strings.LastIndex(href, ".html")
	var subStr = href[preIdx+1 : endIdx]
	value, err := strconv.Atoi(subStr)
	if err != nil {
		return -1
	}
	return value
}

func DownloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 发起 HTTP 请求
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 判断服务器返回状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 将响应内容写入文件
	_, err = io.Copy(out, resp.Body)
	return err
}

func PrepareDirs() {
	fmt.Println("Prepare dirs.")
	os.Mkdir("data", 0777)
	os.Mkdir("imgs", 0777)
}

func FetchPage(pageUrl string) []Ablum {
	ablums := fetchAblums(pageUrl)
	var wg sync.WaitGroup

	for i := range ablums {
		wg.Add(1)
		go ablums[i].FillAndDownloadImages(&wg)
	}
	wg.Wait()
	return ablums
}

func main() {
	PrepareDirs()
	var ablumsList = make([]Ablum, 0, 2048)
	const TotalSize = 38
	for i := TotalSize; i >= 1; i-- {
		var url string
		if i == 1 {
			url = "http://www.mzitu.cc"
		} else {
			url = fmt.Sprintf("http://www.mzitu.cc/xinggan/index_%d.html", i)
		}

		fmt.Println("Progress : ", i-TotalSize, "/", TotalSize)
		fmt.Println("Catch page : ", url)
		sublist := FetchPage(url)
		ablumsList = append(ablumsList, sublist...)
	}

	jsonData, _ := json.Marshal(ablumsList)
	os.WriteFile("data/all.json", jsonData, 0777)
}
