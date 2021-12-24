package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var maxlost = flag.Int("maxlost", 1, "Max lost detections")
var ioutr = flag.Float64("ioutr", 0.2, "IOU threshold")

var trackerCash = map[int64]*Tracker{}

func main() {

	// open output file
	fo, err := os.Create("debug.txt")
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()
	// make a write buffer
	w := bufio.NewWriter(fo)
	logrus.SetLevel(logrus.DebugLevel)
	s := bufio.NewScanner(os.Stdin)
	bufsize := 10 << 20
	buf := make([]byte, bufsize)
	s.Buffer(buf, bufsize)
	for {
		if s.Scan() {
			reqdata := s.Bytes()
			if _, err := w.Write(reqdata); err != nil {
				panic(err)
			}
			image_width := gjson.ParseBytes(reqdata).Get("info.width").Float()
			image_height := gjson.ParseBytes(reqdata).Get("info.height").Float()
			cam_id := gjson.ParseBytes(reqdata).Get("camera.%did").Int()
			if tracker, ok := trackerCash[cam_id]; ok {
				do_track(reqdata, tracker, image_width, image_height)
			} else {
				tracker := NewTracker(*maxlost, *ioutr)
				trackerCash[cam_id] = &tracker
				do_track(reqdata, &tracker, image_width, image_height)
			}
		}

	}
}

func do_track(reqdata []byte, tracker *Tracker, width float64, height float64) {
	items := gjson.ParseBytes(reqdata).Get("items")
	track_items := []gjson.Result{}
	track_items = append(track_items, items.Array()...)
	itemzzzzz, _ := tracker.Updatez(track_items, width, height)
	test := fmt.Sprintf("%s", itemzzzzz)
	reqdata_st, _ := sjson.SetRaw(string(reqdata), "items", test)
	fmt.Println(string(reqdata_st))

}
