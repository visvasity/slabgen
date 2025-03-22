// Copyright (c) 2025 Visvasity LLC

package blockgen

type SortHelper struct {
	LenFunc     func() int
	SwapFunc    func(int, int)
	CompareFunc func(int, int) int
}

func (s *SortHelper) Len() int {
	return s.LenFunc()
}

func (s *SortHelper) Swap(i, j int) {
	s.SwapFunc(i, j)
}

func (s *SortHelper) Less(i, j int) bool {
	return s.CompareFunc(i, j) < 0
}
