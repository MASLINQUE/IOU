package main

import (
	"math"
)

func mathBboxes(bbox1 []float64, bbox2 []float64) bool {
	if bbox1[0] == bbox2[0] {
		if bbox1[1] == bbox2[1] {
			if bbox1[2] == bbox2[2] {
				if bbox1[3] == bbox2[3] {
					return true
				}
			}
		}
	}
	return false
}

//IOU Computes IUO (Intersection Over Union) between two bboxes in the form [x1,y1,x2,y2]
func IOU(bbox1 []float64, bbox2 []float64) float64 {
	xx1 := math.Max(bbox1[0], bbox2[0])
	yy1 := math.Max(bbox1[1], bbox2[1]) //was Max
	xx2 := math.Min(bbox1[2], bbox2[2])
	yy2 := math.Min(bbox1[3], bbox2[3]) //was Min

	interArea := math.Max(0., xx2-xx1+1) * math.Max(0, yy2-yy1+1)
	bbox1Area := (bbox1[2] - bbox1[0] + 1) * (bbox1[3] - bbox1[1] + 1)
	bbox2Area := (bbox2[2] - bbox2[0] + 1) * (bbox2[3] - bbox2[1] + 1)

	iou := interArea / (bbox1Area + bbox2Area - interArea)

	if math.IsNaN(iou) {
		iou = 0
	}

	return iou
}

//RatioMatch computes how close the bbox dimensions from the two bboxes are (0-1). 1-perfect match
func RatioMatch(bbox1 []float64, bbox2 []float64) float64 {
	w1 := (bbox1[2] - bbox1[0])
	h1 := (bbox1[3] - bbox1[1])
	w2 := (bbox2[2] - bbox2[0])
	h2 := (bbox2[3] - bbox2[1])
	r := (w1 / h1) / (w2 / h2)
	if math.IsNaN(r) {
		return 0
	}
	if r > 1 {
		return 1 / r
	}
	return r
}

//AreaMatch computes how close the areas from the two boxes are (0-1). 1-perfect match
func AreaMatch(bbox1 []float64, bbox2 []float64) float64 {
	r := Area(bbox1) / Area(bbox2)
	if math.IsNaN(r) {
		return 0
	}
	if r > 1 {
		return 1 / r
	}
	return r
}

//Area calculates area of a bounding box
func Area(bbox []float64) float64 {
	a := bbox[2] - bbox[0]
	b := bbox[3] - bbox[1]
	return math.Abs(a * b)
}

//ResizeFromCenter resizes a bounding box by a scale factor from its center
func ResizeFromCenter(bbox []float64, scale float64) []float64 {
	w := (bbox[2] - bbox[0])
	h := (bbox[3] - bbox[1])
	dx := (scale*w - w) / 2.0
	dy := (scale*h - h) / 2.0
	// fmt.Printf("bbox %v %f %f", bbox, dx, dy)
	bbox2 := make([]float64, 4)
	bbox2[0] = math.Max(bbox[0]-dx, 0)
	bbox2[1] = math.Max(bbox[1]-dy+h, 0)
	bbox2[2] = math.Min(bbox[2]+dx, 99999)
	bbox2[3] = math.Min(bbox[3]+dy+h, 99999)
	return bbox2
}
