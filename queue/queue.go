package queue

import (
	"fmt"

	"github.com/ritwik310/torrent-client/torrent"
)

// NewQueue .
func NewQueue(ps []*torrent.Piece) *Queue {
	return &Queue{left: 0, right: len(ps) - 1, pieces: ps}
}

// leftest <- left <- right <- rightest

// Queue .
type Queue struct {
	left   int
	right  int
	pieces []*torrent.Piece
}

// Pop .
func (q *Queue) Pop() *torrent.Piece {
	defer func(q *Queue) { q.left++ }(q)
	return q.pieces[q.left]
}

// Push .
func (q *Queue) Push() *torrent.Piece {
	defer func(q *Queue) { q.right-- }(q)
	return q.pieces[q.right]
}

// Elem .
func (q *Queue) Elem(i int) (*torrent.Piece, error) {
	if q.left+i > len(q.pieces) {
		return nil, fmt.Errorf("index out of range")
	}
	return q.pieces[q.left+i], nil
}

// Len .
func (q *Queue) Len() int {
	return len(q.pieces)
}
