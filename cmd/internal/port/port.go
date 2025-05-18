package port

import "fmt"

type Service struct {
	portsAllocated map[int]interface{}
	basePort       int
	portCount      int
	limit          int
}

func New(limit int) *Service {
	allocated := make(map[int]interface{})
	return &Service{
		basePort:       8080,
		portsAllocated: allocated,
		portCount:      0,
		limit:          limit,
	}
}

func (s *Service) GetBasePort() int {
	return s.basePort
}

func (s *Service) GetNext() (int, error) {
	count := s.portCount + 1

	if count > s.limit {
		return 0, fmt.Errorf("too many ports allocated")
	}

	nextPort := s.basePort + count
	s.portsAllocated[nextPort] = nil
	s.portCount = count
	return nextPort, nil
}
