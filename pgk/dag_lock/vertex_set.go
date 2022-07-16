package internal

type VertexSet map[*Vertex]struct{}

func (s VertexSet) Add(v *Vertex) {
	s[v] = struct{}{}
}

func (s VertexSet) Remove(v *Vertex) {
	delete(s, v)
}

func (s VertexSet) Contains(v *Vertex) bool {
	_, ok := s[v]
	return ok
}

func (s VertexSet) Clear() {
	for v := range s {
		delete(s, v)
	}
}
