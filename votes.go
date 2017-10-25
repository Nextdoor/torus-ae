package torus

import (
	"errors"
	"fmt"
)

type voteDirection int

const (
	none = iota
	up
	down
)

const (
	UP_STRING   = "up"
	DOWN_STRING = "down"
	NONE_STRING = "none"
)

var ErrInvalidString = errors.New("invalid vote direction string")

func FromString(value string) (voteDirection, error) {
	switch value {
	case UP_STRING:
		return up, nil
	case DOWN_STRING:
		return down, nil
	case NONE_STRING:
		return none, nil
	default:
		return 0, ErrInvalidString
	}
}

func (v voteDirection) ToString() string {
	switch v {
	case up:
		return UP_STRING
	case down:
		return DOWN_STRING
	case none:
		return NONE_STRING
	default:
		panic(fmt.Sprintf("not a valid vote direction: %v", v))
	}
}

func scoreAdjustment(old, new voteDirection) int {
	var result int

	switch old {
	case up:
		result = -1
	case down:
		result = 1
	case none:
		break
	default:
		panic(fmt.Sprintf("invalid old vote direction: %v", old))
	}

	switch new {
	case up:
		result += 1
	case down:
		result -= 1
	case none:
		break
	default:
		panic(fmt.Sprintf("invalid new vote direction: %v", old))
	}

	return result
}
