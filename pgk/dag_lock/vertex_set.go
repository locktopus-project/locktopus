package internal

type VertexSet map[*Vertice]struct{}

func (s VertexSet) Add(v *Vertice) {
	s[v] = struct{}{}
}

func (s VertexSet) Remove(v *Vertice) {
	delete(s, v)
}

func (s VertexSet) Contains(v *Vertice) bool {
	_, ok := s[v]
	return ok
}

func (s VertexSet) Len() int {
	return len(s)
}

func (s VertexSet) Clear() {
	for v := range s {
		delete(s, v)
	}
}
