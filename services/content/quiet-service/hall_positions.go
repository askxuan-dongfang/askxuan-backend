package main

const worldMinX, worldMaxX, worldMinZ, worldMaxZ = -42, 42, -50, 34

// Keep these public entrance positions aligned with H5 quietPlaces.ts.
var hallApproaches = map[string][2]float64{
	"han-main": {0, -31}, "han-guanyin": {-12, -17}, "han-wenshu": {12, -17},
	"tibetan-buddha": {-28, -13}, "tibetan-guanyin": {-28, -29}, "tibetan-tara": {-28, 3},
	"theravada-buddha": {28, -15}, "theravada-reclining": {28, -32},
	"dao-sanqing": {28, 13}, "dao-caishen": {28, 30}, "dao-zhenwu": {14, 30},
	"mazu-tianhou": {-28, 25},
}

var seatPositions = func() [30][2]float64 {
	var seats [30][2]float64
	copy(seats[:], [][2]float64{{-2, -2}, {0, -2}, {2, -2}, {-2, 0}, {0, 0}, {2, 0}})
	order := []string{"han-main", "han-guanyin", "han-wenshu", "tibetan-buddha", "tibetan-guanyin", "tibetan-tara", "theravada-buddha", "theravada-reclining", "dao-sanqing", "dao-caishen", "dao-zhenwu", "mazu-tianhou"}
	for index, id := range order {
		approach := hallApproaches[id]
		seats[6+index*2] = [2]float64{approach[0] - 1.2, approach[1] - 3.5}
		seats[7+index*2] = [2]float64{approach[0] + 1.2, approach[1] - 3.5}
	}
	return seats
}()
