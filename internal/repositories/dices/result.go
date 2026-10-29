package dices

type DieResult struct {
	Die   Die    `json:"die"`
	Rolls []uint `json:"rolls"`
}

type ThrowResult struct {
	Dices []DieResult `json:"dices"`
	Total int         `json:"total"`
}
