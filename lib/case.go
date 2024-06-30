package lib

type Case struct {
	IndexData
	Data   []byte
	Fields map[string]string
	Logs   map[string]map[string]string
	Scored map[string]map[string]float64
}
