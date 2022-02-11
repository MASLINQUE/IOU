package main

import (
	"fmt"

	"github.com/cpmech/gosl/graph"
	"github.com/tidwall/gjson"
)

type Tracker struct {
	maxLost      int
	iouThreshold float64
	FrameCount   int
	Tracks       []*Track
	nextId       int
}

type Track struct {
	Bbox     []float64
	Prob     float64
	ID       int
	Lost     int
	Age      int
	IOUScore float64
	FrameId  int
}

func MatchBboxes(curr_track *Track, dets [][]float64) (int, Track) {
	// TODO repair logic
	best_match := Track{}
	iou_curr := 0.0
	c_bbox := []float64{dets[0][0], dets[0][1], dets[0][0] + dets[0][2], dets[0][1] + dets[0][3]}
	best_ind := 0
	best_prob := 0.0
	for ind, bbox := range dets {
		prob := bbox[4]
		bbox = []float64{bbox[0], bbox[1], bbox[0] + bbox[2], bbox[1] + bbox[3]}
		if IOU(curr_track.Bbox, bbox) > float64(iou_curr) {
			iou_curr = IOU(curr_track.Bbox, bbox)
			c_bbox = bbox
			best_ind = ind
			best_prob = prob

		}
	}
	best_match.Bbox = c_bbox
	best_match.Prob = best_prob
	best_match.ID = curr_track.ID
	return best_ind, best_match
}

func NewTracker(maxLost int, iouThreshold float64) Tracker {
	return Tracker{
		maxLost:      maxLost,
		iouThreshold: iouThreshold,
		FrameCount:   0,
		nextId:       1,
	}
}

// func (s *Track) Update(frame int, newTrack Track, iouScore float64) {
// 	s.Bbox = newTrack.Bbox
// 	s.ID = newTrack.ID
// 	s.Prob = newTrack.Prob
// 	s.IOUScore = iouScore
// 	s.Age = s.Age + 1

// }

func (s *Track) Update(bbox []float64, prob float64) {
	s.Bbox = bbox
	s.Age = s.Age + 1
	s.Prob = prob

}

func RemoveByIndex(s [][]float64, index int) [][]float64 {
	ret := make([][]float64, 0)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

func (s *Tracker) RemoveTrackByIndexs(indxs []int) {
	var resultVec = []*Track{}

	for _, indx := range indxs {
		s.Tracks[indx] = nil
	}
	for _, val := range s.Tracks {
		if val != nil {
			resultVec = append(resultVec, val)
		}
	}
	s.Tracks = resultVec

}
func (s *Tracker) AddTrack(bbox []float64, prob float64) {
	new_track := Track{}
	new_track.Bbox = bbox
	new_track.Prob = prob
	new_track.ID = s.nextId
	new_track.FrameId = s.FrameCount
	s.nextId = s.nextId + 1
	s.Tracks = append(s.Tracks, &new_track)
}

func (s *Tracker) Update(items []gjson.Result) ([][]float64, [][]float64) {

	dets := [][]float64{}
	for _, item := range items {
		bbox := []float64{}
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox = append(bbox, value.Num)
			return true
		})
		bbox = []float64{bbox[0], bbox[1], bbox[0] + bbox[2], bbox[1] + bbox[3]}
		bbox_prob := append(bbox, item.Get("prob").Num)
		dets = append(dets, bbox_prob)
	}
	bboxesAndID := [][]float64{}

	s.FrameCount = s.FrameCount + 1

	matched, unmatchedDets, unmatchedTrks := associateDetectionsToTrackers(dets, s.Tracks, s.iouThreshold)

	for t := range s.Tracks {
		tracker := s.Tracks[t]

		if !contains(unmatchedTrks, t) {
			for _, det := range matched {
				if det[1] == t {

					bbox := dets[det[0]]
					tracker.Update(bbox[:len(bbox)-1], bbox[len(bbox)-1])

					bboxID := bbox[:len(bbox)-1]
					bboxID = append(bboxID, float64(tracker.ID))
					bboxesAndID = append(bboxesAndID, bboxID)
					break
				}
			}
		}
	}

	// create and initialise new trackers for unmatched detections
	for _, udet := range unmatchedDets {

		s.AddTrack(dets[udet][:4], dets[udet][4])
		ls_bbox := s.Tracks[len(s.Tracks)-1].Bbox
		ls_bbox = append(ls_bbox, float64(s.Tracks[len(s.Tracks)-1].ID))

		bboxesAndID = append(bboxesAndID, ls_bbox)
	}
	delete_ind := []int{}
	for _, i := range unmatchedTrks {
		trk := s.Tracks[i]
		trk.Lost++

		if trk.Lost > s.maxLost {
			delete_ind = append(delete_ind, i)
		}
	}
	if len(delete_ind) > 0 {
		s.RemoveTrackByIndexs(delete_ind)
	}

	ct := ""
	for _, v := range s.Tracks {
		ct = ct + fmt.Sprintf("[id=%d bbox=%v updates=%d] ", v.ID, v.Bbox, v.Age)
	}

	return bboxesAndID, dets
}

func contains(list []int, value int) bool {
	found := false
	for _, v := range list {
		if v == value {
			found = true
			break
		}
	}
	return found
}

func associateDetectionsToTrackers(detections [][]float64, trackers []*Track, iouThreshold float64) ([][]int, []int, []int) {

	if len(trackers) == 0 {
		det := make([]int, 0)
		for i := range detections {
			det = append(det, i)
		}
		return [][]int{}, det, []int{}
	}

	ld := len(detections)
	lt := len(trackers)

	if ld == 0 {
		unmatchedTrackers := make([]int, 0)
		for t := 0; t < lt; t++ {
			unmatchedTrackers = append(unmatchedTrackers, t)
		}
		return [][]int{}, []int{}, unmatchedTrackers
	}

	mk := graph.Munkres{}
	mk.Init(int(ld), int(lt))
	ious := make([][]float64, ld)
	for i := 0; i < len(ious); i++ {
		ious[i] = make([]float64, lt)
	}
	// ============================================================
	for d := 0; d < ld; d++ {
		for t := 0; t < lt; t++ {
			trk := trackers[t]

			//use simple last bbox if not enough updates in this tracker
			tbbox := trk.Bbox
			v := IOU(detections[d], tbbox) //+ AreaMatch(detections[d], tbbox1) + RatioMatch(detections[d], tbbox1)

			ious[d][t] = 1 - v
		}
	}

	//calculate best DETECTION vs TRACKER matches according to COST matrix
	mk.SetCostMatrix(ious)
	mk.Run()
	matchedIndices := [][]int{}
	for i, j := range mk.Links {
		if j != -1 {
			matchedIndices = append(matchedIndices, []int{i, j})
		}
	}

	unmatchedDetections := make([]int, 0)
	for d := 0; d < ld; d++ {
		found := false
		for _, v := range matchedIndices {
			if d == v[0] {
				found = true
				break
			}
		}
		if !found {
			unmatchedDetections = append(unmatchedDetections, d)
		}
	}

	unmatchedTrackers := make([]int, 0)
	for t := 0; t < lt; t++ {
		found := false
		for _, v := range matchedIndices {
			if t == v[1] {
				found = true
				break
			}
		}
		if !found {
			unmatchedTrackers = append(unmatchedTrackers, t)
		}
	}

	matches := make([][]int, 0)
	for _, mi := range matchedIndices {
		//filter out matched with low IOU
		iou := 1 - ious[mi[0]][mi[1]]
		if iou < iouThreshold {
			unmatchedDetections = append(unmatchedDetections, mi[0])
			unmatchedTrackers = append(unmatchedTrackers, mi[1])
		} else {
			matches = append(matches, []int{mi[0], mi[1]})
		}
	}

	return matches, unmatchedDetections, unmatchedTrackers
}
