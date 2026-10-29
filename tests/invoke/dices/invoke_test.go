package dices_test

import (
	"fmt"
	"testing"
)

func TestRollDice_IsNotIdempotent(t *testing.T) {
	if rollDiceTool == nil {
		t.Fatal("roll_dice tool was not registered")
	}
	if rollDiceTool.IsIdempotent() {
		t.Fatal("roll_dice must declare IsIdempotent() == false")
	}
}

func TestInvoke_E2E01_SimpleD20(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-01",
		"Брось один двадцатигранный кубик. Используй roll_dice и сообщи результат.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	calls := toolsNamed(seq, "roll_dice")
	if len(calls) != 1 {
		t.Fatalf("want exactly 1 roll_dice call, got %d (%v)", len(calls), toolNames(seq))
	}

	args := parseRollArgs(t, calls[0].Args)
	if len(args.Groups) != 1 {
		t.Fatalf("ArgumentError: want 1 group, got %d (%s)", len(args.Groups), calls[0].Args)
	}
	g := args.Groups[0]
	if g.Sides != 20 || g.Count != 1 {
		t.Fatalf("ArgumentError: want 1d20, got sides=%d count=%d", g.Sides, g.Count)
	}

	result := parseRollResult(t, calls[0].Result)
	if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 1 {
		t.Fatalf("ExecutionError: unexpected result %+v", result)
	}
	assertPlainFacesInRange(t, result.Dice[0].Rolls, 20)
	if result.Total != int(result.Dice[0].Rolls[0]) {
		t.Fatalf("ExecutionError: total=%d want face=%d", result.Total, result.Dice[0].Rolls[0])
	}

	requireMessageMentions(t, report.ID, result.Total)
	fmt.Printf("E2E-01 PASS tools=%d elapsed=%s total=%d\n", len(seq), report.Elapsed, result.Total)
}

func TestInvoke_E2E02_Modifier(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-02",
		"Сделай проверку характеристики: брось 1d20+5. Используй roll_dice. Покажи результат кубика и итоговую сумму.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	calls := toolsNamed(seq, "roll_dice")
	if len(calls) < 1 {
		t.Fatal("expected roll_dice")
	}
	args := parseRollArgs(t, calls[0].Args)
	g, ok := findGroupBySides(args, 20)
	if !ok || g.Count != 1 {
		t.Fatalf("ArgumentError: want 1d20 group, got %s", calls[0].Args)
	}
	if g.Modifier != 5 {
		t.Fatalf("ArgumentError: want modifier +5, got %d (%s)", g.Modifier, calls[0].Args)
	}

	result := parseRollResult(t, calls[0].Result)
	if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 1 {
		t.Fatalf("ExecutionError: result=%+v", result)
	}
	face := int(result.Dice[0].Rolls[0])
	assertPlainFacesInRange(t, result.Dice[0].Rolls, 20)
	if result.Total != face+5 {
		t.Fatalf("ExecutionError: total=%d want %d", result.Total, face+5)
	}

	requireMessageMentions(t, report.ID, face, result.Total)
	fmt.Printf("E2E-02 PASS tools=%d elapsed=%s face=%d total=%d\n", len(seq), report.Elapsed, face, result.Total)
}

func TestInvoke_E2E03_MultiGroupDamage(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-03",
		"Мой плут попал рапирой и применил скрытую атаку. Брось урон 1d8+3d6+4 через roll_dice одним вызовом. Покажи общий урон.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	calls := toolsNamed(seq, "roll_dice")
	args := parseRollArgs(t, calls[0].Args)
	if len(args.Groups) != 2 {
		t.Fatalf("ArgumentError: want 2 groups (1d8 and 3d6), got %d (%s)", len(args.Groups), calls[0].Args)
	}

	d8, ok8 := findGroupBySides(args, 8)
	d6, ok6 := findGroupBySides(args, 6)
	if !ok8 || d8.Count != 1 {
		t.Fatalf("ArgumentError: missing 1d8 in %s", calls[0].Args)
	}
	if !ok6 || d6.Count != 3 {
		t.Fatalf("ArgumentError: missing 3d6 in %s", calls[0].Args)
	}
	if d8.Modifier+d6.Modifier != 4 {
		t.Fatalf("ArgumentError: modifier +4 must appear once across groups, got d8=%d d6=%d",
			d8.Modifier, d6.Modifier)
	}

	result := parseRollResult(t, calls[0].Result)
	rollCount := 0
	for _, g := range result.Dice {
		rollCount += len(g.Rolls)
		assertPlainFacesInRange(t, g.Rolls, g.Sides)
	}
	if rollCount != 4 {
		t.Fatalf("ExecutionError: want 4 dice faces, got %d (%+v)", rollCount, result)
	}
	if result.Total != sumRolls(result)+4 {
		t.Fatalf("ExecutionError: total=%d facesSum=%d want faces+4=%d",
			result.Total, sumRolls(result), sumRolls(result)+4)
	}

	requireMessageMentions(t, report.ID, result.Total)
	fmt.Printf("E2E-03 PASS tools=%d elapsed=%s total=%d\n", len(seq), report.Elapsed, result.Total)
}

func TestInvoke_E2E04_Advantage(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-04",
		"Сделай проверку с преимуществом через roll_dice. Брось два d20, оставь больший результат (keep highest 1) и прибавь 3. Сообщи итог.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	args := parseRollArgs(t, firstTool(seq, "roll_dice").Args)
	g, ok := findGroupBySides(args, 20)
	if !ok || g.Count != 2 {
		t.Fatalf("ArgumentError: want 2d20, got %s", firstTool(seq, "roll_dice").Args)
	}
	if g.Keep.Highest != 1 {
		t.Fatalf("ArgumentError: want keep highest_count=1, got %+v", g.Keep)
	}
	if g.Modifier != 3 {
		t.Fatalf("ArgumentError: want modifier +3, got %d", g.Modifier)
	}

	result := parseRollResult(t, firstTool(seq, "roll_dice").Result)
	if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 1 {
		t.Fatalf("ExecutionError: want 1 kept roll, got %+v", result)
	}
	kept := int(result.Dice[0].Rolls[0])
	assertPlainFacesInRange(t, result.Dice[0].Rolls, 20)
	if result.Total != kept+3 {
		t.Fatalf("ExecutionError: total=%d want %d", result.Total, kept+3)
	}

	requireMessageMentions(t, report.ID, result.Total)
	fmt.Printf("E2E-04 PASS tools=%d elapsed=%s kept=%d total=%d\n", len(seq), report.Elapsed, kept, result.Total)
}

func TestInvoke_E2E05_Disadvantage(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-05",
		"Сделай проверку с помехой через roll_dice. Брось два d20, оставь меньший результат (keep lowest 1) и прибавь 2. Сообщи итог.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	args := parseRollArgs(t, firstTool(seq, "roll_dice").Args)
	g, ok := findGroupBySides(args, 20)
	if !ok || g.Count != 2 {
		t.Fatalf("ArgumentError: want 2d20, got %s", firstTool(seq, "roll_dice").Args)
	}
	if g.Keep.Lowest != 1 {
		t.Fatalf("ArgumentError: want keep lowest_count=1, got %+v", g.Keep)
	}
	if g.Modifier != 2 {
		t.Fatalf("ArgumentError: want modifier +2, got %d", g.Modifier)
	}

	result := parseRollResult(t, firstTool(seq, "roll_dice").Result)
	if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 1 {
		t.Fatalf("ExecutionError: want 1 kept roll, got %+v", result)
	}
	kept := int(result.Dice[0].Rolls[0])
	assertPlainFacesInRange(t, result.Dice[0].Rolls, 20)
	if result.Total != kept+2 {
		t.Fatalf("ExecutionError: total=%d want %d", result.Total, kept+2)
	}

	requireMessageMentions(t, report.ID, result.Total)
	fmt.Printf("E2E-05 PASS tools=%d elapsed=%s kept=%d total=%d\n", len(seq), report.Elapsed, kept, result.Total)
}

func TestInvoke_E2E06_ComplexRerollKeep(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-06",
		"Брось 4d6 через roll_dice. Каждый кубик может один раз перебросить единицу (reroll equal 1, max_rerolls 1). Затем оставь три наибольших результата (keep highest 3) и сообщи сумму.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	args := parseRollArgs(t, firstTool(seq, "roll_dice").Args)
	g, ok := findGroupBySides(args, 6)
	if !ok || g.Count != 4 {
		t.Fatalf("ArgumentError: want 4d6, got %s", firstTool(seq, "roll_dice").Args)
	}
	if g.Reroll.Equal != 1 || g.Reroll.Count != 1 {
		t.Fatalf("ArgumentError: want reroll equal=1 max=1, got %+v", g.Reroll)
	}
	if g.Keep.Highest != 3 {
		t.Fatalf("ArgumentError: want keep highest 3, got %+v", g.Keep)
	}

	result := parseRollResult(t, firstTool(seq, "roll_dice").Result)
	if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 3 {
		t.Fatalf("ExecutionError: want 3 kept rolls, got %+v", result)
	}
	assertPlainFacesInRange(t, result.Dice[0].Rolls, 6)
	if result.Total != sumRolls(result) {
		t.Fatalf("ExecutionError: total=%d want faces sum=%d", result.Total, sumRolls(result))
	}

	requireMessageMentions(t, report.ID, result.Total)
	fmt.Printf("E2E-06 PASS tools=%d elapsed=%s rolls=%v total=%d\n",
		len(seq), report.Elapsed, result.Dice[0].Rolls, result.Total)
}

func TestInvoke_E2E07_NoRollNeeded(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-07",
		"Сколько граней у стандартного кубика d20? Ничего не бросай. Ответь через send_message.")

	seq := report.ToolCalls
	requireNoTool(t, seq, "roll_dice")
	requireTool(t, seq, "send_message")
	requireMessageMentions(t, report.ID, 20)
	fmt.Printf("E2E-07 PASS tools=%d elapsed=%s\n", len(seq), report.Elapsed)
}

func TestInvoke_E2E08_TwoIndependentRolls(t *testing.T) {
	resetDB(t)

	report := invoke(t, "E2E-08",
		"Брось один d20 для атаки и отдельно 2d6+3 для урона через roll_dice. Сообщи оба результата (атака и урон) через send_message.")

	seq := report.ToolCalls
	requireTool(t, seq, "roll_dice")
	requireRollCompleted(t, report)

	calls := toolsNamed(seq, "roll_dice")
	var attackTotal, damageTotal int
	var sawAttack, sawDamage bool

	for _, call := range calls {
		args := parseRollArgs(t, call.Args)
		result := parseRollResult(t, call.Result)
		g20, has20 := findGroupBySides(args, 20)
		g6, has6 := findGroupBySides(args, 6)

		// One call covering both logical rolls.
		if len(args.Groups) >= 2 && has20 && g20.Count == 1 && has6 && g6.Count == 2 {
			mod := 0
			for _, grp := range args.Groups {
				mod += grp.Modifier
			}
			if mod != 3 {
				t.Fatalf("ArgumentError: combined damage want +3, got mod=%d (%s)", mod, call.Args)
			}
			if len(result.Dice) != 2 {
				t.Fatalf("combined result unexpected: %+v", result)
			}
			for _, dg := range result.Dice {
				assertPlainFacesInRange(t, dg.Rolls, dg.Sides)
			}
			for _, dg := range result.Dice {
				switch dg.Sides {
				case 20:
					if len(dg.Rolls) != 1 {
						t.Fatalf("attack faces=%v", dg.Rolls)
					}
					attackTotal = int(dg.Rolls[0])
					sawAttack = true
				case 6:
					if len(dg.Rolls) != 2 {
						t.Fatalf("damage faces=%v", dg.Rolls)
					}
					damageTotal = 0
					for _, face := range dg.Rolls {
						damageTotal += int(face)
					}
					damageTotal += 3
					sawDamage = true
				}
			}
			if result.Total != attackTotal+damageTotal {
				t.Fatalf("combined total=%d want attack+damage=%d+%d", result.Total, attackTotal, damageTotal)
			}
			continue
		}

		if has20 && g20.Count == 1 && len(args.Groups) == 1 {
			if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 1 {
				t.Fatalf("attack result unexpected: %+v", result)
			}
			assertPlainFacesInRange(t, result.Dice[0].Rolls, 20)
			if result.Total != int(result.Dice[0].Rolls[0])+g20.Modifier {
				t.Fatalf("attack total=%d face=%d mod=%d", result.Total, result.Dice[0].Rolls[0], g20.Modifier)
			}
			attackTotal = result.Total
			sawAttack = true
			continue
		}

		if has6 && g6.Count == 2 && len(args.Groups) == 1 {
			if g6.Modifier != 3 {
				t.Fatalf("ArgumentError: damage want +3, got %d (%s)", g6.Modifier, call.Args)
			}
			if len(result.Dice) != 1 || len(result.Dice[0].Rolls) != 2 {
				t.Fatalf("damage want 2 faces, got %+v", result)
			}
			assertPlainFacesInRange(t, result.Dice[0].Rolls, 6)
			if result.Total != sumRolls(result)+3 {
				t.Fatalf("damage total=%d faces=%d", result.Total, sumRolls(result))
			}
			damageTotal = result.Total
			sawDamage = true
			continue
		}

		t.Fatalf("ArgumentError: unrecognized roll_dice args %s", call.Args)
	}

	if !sawAttack || !sawDamage {
		t.Fatalf("ArgumentError: missing attack and/or damage roll in calls %#v", toolNames(calls))
	}

	requireMessageMentions(t, report.ID, attackTotal, damageTotal)
	fmt.Printf("E2E-08 PASS tools=%d elapsed=%s attack=%d damage=%d\n",
		len(seq), report.Elapsed, attackTotal, damageTotal)
}
