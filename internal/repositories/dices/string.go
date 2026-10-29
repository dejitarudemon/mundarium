package dices

import (
	"fmt"
	"strconv"
	"strings"
)

// String returns a compact keep descriptor, e.g. "kh3", "kl2", "kh3kl1".
func (k Keep) String() string {
	var b strings.Builder
	if k.Highest > 0 {
		b.WriteString("kh")
		b.WriteString(strconv.FormatUint(uint64(k.Highest), 10))
	}
	if k.Lowest > 0 {
		b.WriteString("kl")
		b.WriteString(strconv.FormatUint(uint64(k.Lowest), 10))
	}
	return b.String()
}

// String returns a compact reroll descriptor, e.g. "r=1", "r<3x2".
func (r Reroll) String() string {
	return formatFaceRule("r", r.Equal, r.Lt, r.Gt, r.Count)
}

// String returns a compact blow descriptor, e.g. "!=6", "!>4x2".
func (b Blow) String() string {
	return formatFaceRule("!", b.Equal, b.Lt, b.Gt, b.Count)
}

// String returns D&D-style dice notation for this die group, e.g. "4d6kh3r<3+2".
// Sub is omitted here; DicesGroup.String renders the leading sign.
func (d Die) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%dd%d", d.Count, d.Sides)

	if keep := d.Keep.String(); keep != "" {
		b.WriteString(keep)
	}
	if reroll := d.Reroll.String(); reroll != "" {
		b.WriteString(reroll)
	}
	if blow := d.Blow.String(); blow != "" {
		b.WriteString(blow)
	}
	if d.Modifier > 0 {
		fmt.Fprintf(&b, "+%d", d.Modifier)
	} else if d.Modifier < 0 {
		fmt.Fprintf(&b, "%d", d.Modifier)
	}
	if d.Multiplier != 0 && d.Multiplier != 1 {
		fmt.Fprintf(&b, "*%d", d.Multiplier)
	}

	return b.String()
}

// String joins dice with "+" / "-" according to Sub, e.g. "2d6+5 - 1d4".
func (g DicesGroup) String() string {
	if len(g.Dices) == 0 {
		return ""
	}

	var b strings.Builder
	for i, die := range g.Dices {
		switch {
		case i == 0 && die.Sub:
			b.WriteByte('-')
			b.WriteString(die.String())
		case i == 0:
			b.WriteString(die.String())
		case die.Sub:
			b.WriteString(" - ")
			b.WriteString(die.String())
		default:
			b.WriteString(" + ")
			b.WriteString(die.String())
		}
	}
	return b.String()
}

// String returns notation and kept faces, e.g. "4d6kh3:[3 6 6]".
func (r DieResult) String() string {
	return r.Die.String() + ":[" + joinUint(r.Rolls) + "]"
}

// String returns all group results and the total, e.g. "4d6:[3 6 6] + 1d4:[2] = 17".
func (r ThrowResult) String() string {
	if len(r.Dices) == 0 {
		return "= 0"
	}

	parts := make([]string, 0, len(r.Dices))
	for _, group := range r.Dices {
		part := group.String()
		if group.Die.Sub {
			part = "-(" + part + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " + ") + " = " + strconv.Itoa(r.Total)
}

func formatFaceRule(prefix string, equal, lt, gt, count uint8) string {
	if equal == 0 && lt == 0 && gt == 0 && count == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(prefix)
	switch {
	case equal != 0:
		b.WriteByte('=')
		b.WriteString(strconv.FormatUint(uint64(equal), 10))
	case lt != 0:
		b.WriteByte('<')
		b.WriteString(strconv.FormatUint(uint64(lt), 10))
	case gt != 0:
		b.WriteByte('>')
		b.WriteString(strconv.FormatUint(uint64(gt), 10))
	}
	if count > 0 {
		b.WriteByte('x')
		b.WriteString(strconv.FormatUint(uint64(count), 10))
	}
	return b.String()
}

func joinUint(values []uint) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(parts, " ")
}
