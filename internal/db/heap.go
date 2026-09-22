package db

type ResultHeap []Result

func (h ResultHeap) Len() int { 
	return len(h) 
}
func (h ResultHeap) Less(i, j int) bool { 
	return h[i].score < h[j].score
}
func (h ResultHeap) Swap(i, j int) { 
	h[i], h[j] = h[j], h[i] 
}

func (h *ResultHeap) Push(r any) {
	res := r.(Result)
	*h = append(*h, res)
}

func (h *ResultHeap) Pop() any{
	old := *h  // reference the pointer to get the slice
	n := len(old)
	l := old[n-1] // get the last element
	*h = old[0 : n-1] // reassign h without last element

	return l
}
