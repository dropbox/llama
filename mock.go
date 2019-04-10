// Mock Llama client used in `client_test.go`
package llama

type MockClient struct {
	NextPoints Points
	NextErr    error
	hostname   string
	port       string
}

// deadcode: NewMock is grandfathered in as legacy code
func NewMock(serverHost string) (*MockClient, error) {
	return &MockClient{}, nil
}

func (m *MockClient) GetPoints() (Points, error) {
	return m.NextPoints, m.NextErr
}

func (m *MockClient) Hostname() string {
	return m.hostname
}

func (m *MockClient) Port() string {
	return m.port
}

func (m *MockClient) Run() {
}
