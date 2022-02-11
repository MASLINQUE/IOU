package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var maxlost = flag.Int("maxlost", 1, "Max lost detections")
var ioutr = flag.Float64("ioutr", 0.2, "IOU threshold")

var trackerCash = map[int64]*Tracker{}

func main() {

	// open output file
	flag.Parse()
	// make a write buffer
	logrus.SetLevel(logrus.DebugLevel)
	s := bufio.NewScanner(os.Stdin)
	bufsize := 10 << 20
	buf := make([]byte, bufsize)
	s.Buffer(buf, bufsize)
	for {
		if s.Scan() {
			reqdata := s.Bytes()
			image_width := gjson.ParseBytes(reqdata).Get("info.width").Float()
			image_height := gjson.ParseBytes(reqdata).Get("info.height").Float()
			cam_id := gjson.ParseBytes(reqdata).Get("camera.%did").Int()

			if tracker, ok := trackerCash[cam_id]; ok {
				track(reqdata, tracker, image_width, image_height)
			} else {
				tracker := NewTracker(*maxlost, *ioutr)
				trackerCash[cam_id] = &tracker
				track(reqdata, &tracker, image_width, image_height)
			}
		}

	}
}

func track(reqdata []byte, tracker *Tracker, width float64, height float64) {

	items := gjson.ParseBytes(reqdata).Get("items")
	if len(items.Array()) == 0 {
		fmt.Println(string(reqdata))
		return
	}
	itemsArray := []gjson.Result{}
	itemsArray = append(itemsArray, items.Array()...)

	bboxesAndIDs, _ := tracker.Update(itemsArray)
	responseStringArray := []string{}

	for _, item := range itemsArray {
		bbox_det := []float64{}
		item_str := item.String()
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox_det = append(bbox_det, value.Num)
			return true
		})
		bbox_det = []float64{bbox_det[0], bbox_det[1], bbox_det[0] + bbox_det[2], bbox_det[1] + bbox_det[3]}
		for _, bbox_trc := range bboxesAndIDs {
			// log.Printf("bbox_det %v, bbox_trc %v, iou -- %v", bbox_det, bbox_trc, mathBboxes(bbox_det, bbox_trc))

			if mathBboxes(bbox_det, bbox_trc) {
				item_str, _ = sjson.Set(item_str, "id", fmt.Sprintf("%.0f", bbox_trc[len(bbox_trc)-1]))
				responseStringArray = append(responseStringArray, item_str)
				break
			}
		}
	}
	responseString := fmt.Sprintf("[%s]", strings.Join(responseStringArray, ", "))

	reqdata_st, _ := sjson.SetRaw(string(reqdata), "items", responseString)
	fmt.Println(string(reqdata_st))

}
