package shiftpad

import "golang.org/x/exp/maps"

func Intersect(as, bs []string) []string {
	if len(as) < len(bs) {
		as, bs = bs, as
	}
	// now a is longer
	bmap := make(map[string]any)
	for _, b := range bs {
		bmap[b] = struct{}{}
	}
	var result []string
	for _, a := range as {
		if _, ok := bmap[a]; ok {
			result = append(result, a)
		}
	}
	return result
}

func Union(as, bs []string) []string {
	if len(as) == 0 {
		return bs
	}
	if len(bs) == 0 {
		return as
	}

	var result = make(map[string]any)
	for _, a := range as {
		result[a] = struct{}{}
	}
	for _, b := range bs {
		result[b] = struct{}{}
	}
	return maps.Keys(result)
}
