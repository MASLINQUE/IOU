package main

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
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

func (s *Track) Update(frame int, newTrack Track, iouScore float64) {
	s.Bbox = newTrack.Bbox
	s.ID = newTrack.ID
	s.Prob = newTrack.Prob
	s.IOUScore = iouScore
	s.Age = s.Age + 1

}

func (s *Track) Updatez(bbox []float64, prob float64) {
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

func (s *Tracker) Update(dets [][]float64) {
	s.FrameCount = s.FrameCount + 1
	updates_track := []int{}
	deleted_tracks := []int{}
	for ind, track := range s.Tracks {
		if len(dets) > 0 {
			index, best_match := MatchBboxes(track, dets)
			iou := IOU(best_match.Bbox, track.Bbox)
			if iou >= s.iouThreshold {
				track.Update(s.FrameCount, best_match, iou)
				updates_track = append(updates_track, track.ID)
				RemoveByIndex(dets, index)
			}
		}
		if len(updates_track) == 0 || track.ID != updates_track[len(updates_track)-1] {
			track.Lost++
			if track.Lost > s.maxLost {
				deleted_tracks = append(deleted_tracks, ind)
			}

		}
	}
	if len(deleted_tracks) > 0 {
		s.RemoveTrackByIndexs(deleted_tracks)
	}

	for _, detect := range dets {
		bbox := []float64{detect[0], detect[1], detect[0] + detect[2], detect[1] + detect[3]}
		prob := detect[4]
		s.AddTrack(bbox, prob)
	}

}

func (s *Tracker) Updatez(items []gjson.Result, width float64, height float64) ([]string, error) {

	dets := [][]float64{}
	for _, item := range items {
		bbox := []float64{}
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox = append(bbox, value.Num)
			return true
		})
		bbox = []float64{bbox[0] * width, bbox[1] * height, bbox[0]*width + bbox[2]*width, bbox[1]*height + bbox[3]*height}
		bbox_prob := append(bbox, item.Get("prob").Num)
		dets = append(dets, bbox_prob)
		// logrus.Debugf("SORT Update dets=%v iouThreshold=%f", bbox_prob, s.iouThreshold)
	}
	items_string := []string{}
	for _, item := range items {
		items_string = append(items_string, item.String())

	}
	s.FrameCount = s.FrameCount + 1

	matched, unmatchedDets, unmatchedTrks := associateDetectionsToTrackers(dets, s.Tracks, s.iouThreshold)

	// logrus.Debugf("Detection X Trackers. matched=%v unmatchedDets=%v unmatchedTrks=%v", matched, unmatchedDets, unmatchedTrks)
	// logrus.Debugf("CHECK %v", 1)
	// update matched trackers with assigned detections

	for t := range s.Tracks {
		tracker := s.Tracks[t]
		// logrus.Debugf("Matched t %v, len Trackers %v, unmatchedTrks %v", t, len(s.Tracks), unmatchedTrks)
		//is this tracker still matched?
		if !contains(unmatchedTrks, t) {
			for _, det := range matched {
				logrus.Debugf("Det %v", det)
				if det[1] == t {
					bbox := dets[det[0]]
					// logrus.Debugf("CHECK %v", 1)
					tracker.Updatez(bbox[:len(bbox)-1], bbox[len(bbox)-1])
					// logrus.Debugf("CHECK %v", 2)
					// pathkey := fmt.Sprintf("id", tracker.ID)
					items_string[det[0]], _ = sjson.Set(items_string[det[0]], "id", strconv.FormatInt(int64(tracker.ID), 10))
					// logrus.Debugf("CHECK %v", 3)
					// reqdata, _ = sjson.SetBytes(reqdata, pathkey, strconv.FormatInt(int64(tracker.Tracks[ind].ID), 10))
					// logrus.Debugf("Tracker updated. id=%d bbox=%v updates=%d\n", tracker.ID, bbox, tracker.Age)
					break
				}
			}
		}
	}

	// create and initialise new trackers for unmatched detections
	for _, udet := range unmatchedDets {
		// logrus.Debugf("CHECK %v", 5)
		// aread := Area(dets[udet])
		// if aread < 1 {
		// 	logrus.Debugf("Ignoring too small detection. bbox=%f area=%f", dets[udet], aread)
		// 	continue
		// }

		s.AddTrack(dets[udet], dets[udet][4])
		// logrus.Debugf("CHECK %v", 6)
		// pathkey := fmt.Sprintf("id", s.Tracks[len(s.Tracks)-1].ID)
		items_string[udet], _ = sjson.Set(items_string[udet], "id", strconv.FormatInt(int64(s.Tracks[len(s.Tracks)-1].ID), 10))
		// logrus.Debugf("CHECK %v", 7)
		// logrus.Debugf("New tracker added. id=%d bbox=%v\n", s.Tracks[len(s.Tracks)-1].ID, s.Tracks[len(s.Tracks)-1].Bbox)
	}
	delete_ind := []int{}
	for _, i := range unmatchedTrks {
		trk := s.Tracks[i]
		trk.Lost++
		// logrus.Debugf("May be delete trackers. id=%d bbox=%v lost=%d\n", trk.ID, trk.Bbox, trk.Lost)
		// logrus.Debugf("CHECK %v", 8)
		if trk.Lost > s.maxLost {
			delete_ind = append(delete_ind, i)
			// s.Tracks = append(s.Tracks[:i], s.Tracks[i+1:]...)
			// logrus.Debugf("Tracker removed. id=%d, bbox=%v updates=%d\n", trk.ID, trk.Bbox, trk.Age)
		}
	}
	if len(delete_ind) > 0 {
		// logrus.Debugf("CHECK %v", 9)
		s.RemoveTrackByIndexs(delete_ind)
		// logrus.Debugf("CHECK %v", 10)
	}

	//remove dead trackers
	// ti := len(s.Tracks)
	// for t := ti - 1; t >= 0; t-- {
	// 	trk := s.Tracks[t]

	// 	trk.Lost++
	// 	logrus.Debugf("May be delete trackers. id=%d bbox=%v lost=%d\n", trk.ID, trk.Bbox, trk.Lost)
	// 	if trk.Lost > s.maxLost+1 {
	// 		s.Tracks = append(s.Tracks[:t], s.Tracks[t+1:]...)
	// 		logrus.Debugf("Tracker removed. id=%d, bbox=%v updates=%d\n", trk.ID, trk.Bbox, trk.Age)
	// 	}
	// }

	ct := ""
	for _, v := range s.Tracks {
		ct = ct + fmt.Sprintf("[id=%d bbox=%v updates=%d] ", v.ID, v.Bbox, v.Age)
	}
	// logrus.Debugf("Current trackers=%s", ct)

	return items_string, nil
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
		// // fmt.Printf(">>>>EMPTY DETECTIONS %d %d", ld, lt)
		return [][]int{}, []int{}, unmatchedTrackers
	}

	// iouMatrix := make([][]float64, ld)

	mk := Munkres{}
	mk.Init(int(ld), int(lt))
	ious := make([][]float64, ld)
	for i := 0; i < len(ious); i++ {
		ious[i] = make([]float64, lt)
	}
	// ============================================================
	for d := 0; d < ld; d++ {
		// iouMatrix[d] = make([]float64, lt)
		for t := 0; t < lt; t++ {
			trk := trackers[t]

			//use simple last bbox if not enough updates in this tracker
			tbbox := trk.Bbox
			v := IOU(detections[d], tbbox) //+ AreaMatch(detections[d], tbbox1) + RatioMatch(detections[d], tbbox1)

			ious[d][t] = 1 - v
		}
	}

	// ============================================================
	// mm := munkres.NewMatrix(ld, lt)
	//initialize IOUS cost matrix
	// ious := make([][]float64, ld)
	// for i := 0; i < len(ious); i++ {
	// 	ious[i] = make([]float64, lt)
	// }

	//calculate best DETECTION vs TRACKER matches according to COST matrix
	mk.SetCostMatrix(ious)
	mk.Run()
	matchedIndices := [][]int{}
	for i, j := range mk.Links {
		if j != -1 {
			matchedIndices = append(matchedIndices, []int{i, j})
		}
	}

	// logrus.Debugf("Detection x Tracker match=%v", matchedIndices)

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
			// logrus.Debugf("Unmatched detection found. bbox=%v", detections[d])
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
		// logrus.Debugf("IOU iou=%v", ious)
		if iou < iouThreshold {
			// logrus.Debugf("Skipping detection/tracker because it has low IOU deti=%d trki=%d iou=%f", mi[0], mi[1], iou)
			unmatchedDetections = append(unmatchedDetections, mi[0])
			unmatchedTrackers = append(unmatchedTrackers, mi[1])
		} else {
			matches = append(matches, []int{mi[0], mi[1]})
		}
	}

	return matches, unmatchedDetections, unmatchedTrackers
}
