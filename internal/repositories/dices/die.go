package dices

type Keep struct {
	Highest uint8 `json:"highest_count"`
	Lowest  uint8 `json:"lowest_count"`
}

type Reroll struct {
	Equal uint8 `json:"condition_equal"`
	Lt    uint8 `json:"condition_lower_than"`
	Gt    uint8 `json:"condition_greater_than"`

	Count uint8 `json:"max_rerolls"`
}

type Blow struct {
	Equal uint8 `json:"condition_equal"`
	Lt    uint8 `json:"condition_lower_than"`
	Gt    uint8 `json:"condition_greater_than"`

	Count uint8 `json:"max_blows"`
}

type Die struct {
	Sides uint8 `json:"sides"`
	Count uint8 `json:"count"`
	Sub   bool  `json:"sub"`

	Keep   Keep   `json:"keep_settings"`
	Reroll Reroll `json:"reroll_setting"`
	Blow   Blow   `json:"blow_setting"`

	Modifier   int `json:"modifier"`
	Multiplier int `json:"multiplier"`
}

type DicesGroup struct {
	Dices []Die `json:"dieces_group"`
}
