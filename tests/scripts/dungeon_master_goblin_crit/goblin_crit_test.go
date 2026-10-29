package dungeon_master_goblin_crit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/providers"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
)

const (
	playerAC      = 12
	playerHPStart = 20
	goblinAtkMod  = 4
)

// TestInvoke_DungeonMaster_GoblinCrit runs a single Invoke where the model acts as
// a D&D 5e DM without any tool-name hints in the prompts. Tools are available only
// via normal API registration.
func TestInvoke_DungeonMaster_GoblinCrit(t *testing.T) {
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	resetToolSeq()

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 600)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	started := time.Now()

	fmt.Printf("\n========== TRACE: DungeonMaster Goblin Crit ==========\n")
	fmt.Printf("model: %s\nurl: %s\nmax_iters: %d\ndice_seed: %d\ninvoke_id: %s\n\n",
		llmModel, llmURL, llmMaxIter, diceRNGSeed, invokeID)
	fmt.Printf("--- System prompt ---\n%s\n\n", systemPrompt)
	fmt.Printf("--- Player message ---\n%s\n\n", playerMessage)

	err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(playerMessage),
	})
	elapsed := time.Since(started)

	seq := snapshotToolSeq()
	fmt.Printf("--- Tool calls (%d) ---\n", len(seq))
	for i, rec := range seq {
		fmt.Printf("%d. %s\n   args:   %s\n   result: %s\n   err:    %v\n",
			i+1, rec.Name, compactJSON(rec.Args), compactJSON(rec.Result), rec.Err)
	}

	var journaled []journal.Request
	if jerr := testConn.DB().Where("group_id = ?", invokeID).Order("created_at asc").Find(&journaled).Error; jerr != nil {
		t.Fatalf("journal: %v", jerr)
	}
	fmt.Printf("\n--- Journal (%d) ---\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s err=%v\n", req.Tool, req.Status, req.Error)
	}

	playerText, hasMsg := lastPlayerMessage(t, invokeID)
	fmt.Printf("\n--- Final player message ---\n")
	if hasMsg {
		fmt.Printf("%s\n", playerText)
	} else {
		fmt.Printf("<none>\n")
	}
	fmt.Printf("\n--- Invoke timing ---\nelapsed=%s err=%v\n", elapsed, err)

	var (
		failures           []string
		toolUsagePass      = true
		rulesReasoningPass = true
		narrativePass      = true
	)

	if err != nil {
		failf(&failures, "Invoke error: %v", err)
		toolUsagePass = false
		rulesReasoningPass = false
		narrativePass = false
	}

	rolls := toolsNamed(seq, "roll_dice")
	msgs := toolsNamed(seq, "send_message")

	natural := 0
	attackTotal := 0
	hit := false
	critical := false
	damage := 0
	damageParams := "отсутствует"
	hpLeft := playerHPStart

	if len(rolls) == 0 {
		failf(&failures, "Tool usage: roll_dice was never called")
		toolUsagePass = false
		rulesReasoningPass = false
	} else {
		attackCall := rolls[0]
		args, aerr := parseRollArgs(attackCall.Args)
		res, rerr := parseRollResult(attackCall.Result)
		if aerr != nil || rerr != nil || attackCall.Err != nil {
			failf(&failures, "Tool usage: attack roll parse/exec error args=%v result=%v exec=%v", aerr, rerr, attackCall.Err)
			toolUsagePass = false
		} else {
			atk, ok := findDie(args, 20, 1)
			if !ok {
				failf(&failures, "Tool usage: first roll_dice is not a 1d20 attack (%s)", attackCall.Args)
				toolUsagePass = false
			} else {
				if atk.Modifier != goblinAtkMod {
					failf(&failures, "Tool usage: attack modifier want +%d, got %d", goblinAtkMod, atk.Modifier)
					toolUsagePass = false
				}
				if atk.Keep.Highest != 0 || atk.Keep.Lowest != 0 {
					failf(&failures, "Tool usage: unexpected keep on attack: %+v", atk.Keep)
					toolUsagePass = false
				}
				if len(res.Dice) != 1 || len(res.Dice[0].Rolls) != 1 {
					failf(&failures, "Tool usage: attack result want one d20 face, got %+v", res)
					toolUsagePass = false
				} else {
					natural = int(res.Dice[0].Rolls[0])
					attackTotal = res.Total
					if attackTotal != natural+atk.Modifier {
						failf(&failures, "Tool usage: attack total=%d inconsistent with face=%d mod=%d",
							attackTotal, natural, atk.Modifier)
						toolUsagePass = false
					}
					switch {
					case natural == 1:
						hit, critical = false, false
					case natural == 20:
						hit, critical = true, true
					default:
						hit = attackTotal >= playerAC
						critical = false
					}
				}
			}
		}
	}

	if natural != 0 {
		if !hit {
			if len(rolls) > 1 {
				failf(&failures, "Rules: miss but extra roll_dice calls after attack (%d total)", len(rolls))
				rulesReasoningPass = false
			}
			damage = 0
			hpLeft = playerHPStart
		} else {
			if len(rolls) < 2 {
				failf(&failures, "Rules: hit/crit but no damage roll_dice call")
				rulesReasoningPass = false
				toolUsagePass = false
			} else {
				dmgCall := rolls[1]
				dargs, aerr := parseRollArgs(dmgCall.Args)
				dres, rerr := parseRollResult(dmgCall.Result)
				if aerr != nil || rerr != nil || dmgCall.Err != nil {
					failf(&failures, "Tool usage: damage roll error args=%v result=%v exec=%v", aerr, rerr, dmgCall.Err)
					toolUsagePass = false
					rulesReasoningPass = false
				} else {
					wantCount := uint8(1)
					if critical {
						wantCount = 2
					}
					g, ok := findDieAnyCount(dargs, 6)
					if !ok || g.Count != wantCount || g.Modifier != 2 {
						failf(&failures, "Rules: damage args want %dd6+2 (crit=%v), got %s",
							wantCount, critical, dmgCall.Args)
						rulesReasoningPass = false
						toolUsagePass = false
					} else if len(dres.Dice) == 0 || len(dres.Dice[0].Rolls) != int(wantCount) {
						failf(&failures, "Rules: damage faces want %d, got %+v", wantCount, dres)
						rulesReasoningPass = false
					} else {
						damageParams = fmt.Sprintf("%dd6+2 → result=%s", wantCount, compactJSON(dmgCall.Result))
						if dres.Total != sumFaces(dres)+g.Modifier {
							failf(&failures, "Rules: damage total=%d faces=%d mod=%d", dres.Total, sumFaces(dres), g.Modifier)
							rulesReasoningPass = false
						}
						damage = dres.Total
						hpLeft = playerHPStart - damage
						if hpLeft < 0 {
							hpLeft = 0
						}
					}
				}
			}
			if len(rolls) > 2 {
				failf(&failures, "Rules: unexpected extra roll_dice calls (%d)", len(rolls))
				rulesReasoningPass = false
			}
		}
	}

	if len(msgs) == 0 || !hasMsg {
		failf(&failures, "Narrative: send_message was not used / no player message")
		narrativePass = false
		toolUsagePass = false
	} else if natural != 0 {
		if lookLikeHit(playerText) && !hit {
			failf(&failures, "Rules/Narrative: model reported hit but attack missed (d20=%d total=%d)", natural, attackTotal)
			rulesReasoningPass = false
			narrativePass = false
		}
		if lookLikeMiss(playerText) && hit {
			failf(&failures, "Rules/Narrative: model reported miss but attack hit (d20=%d total=%d)", natural, attackTotal)
			rulesReasoningPass = false
			narrativePass = false
		}
		if !lookLikeHit(playerText) && !lookLikeMiss(playerText) {
			failf(&failures, "Narrative: cannot detect hit/miss wording in player message")
			narrativePass = false
		}
		if hit && !messageMentionsInt(playerText, damage) {
			failf(&failures, "Narrative: hit message does not mention damage %d", damage)
			narrativePass = false
		}
		if !messageMentionsInt(playerText, natural) && !messageMentionsInt(playerText, attackTotal) {
			fmt.Printf("note: player message omits natural d20=%d and attack total=%d\n", natural, attackTotal)
		}
	}

	hitLabel := "нет"
	if critical {
		hitLabel = "критическое"
	} else if hit {
		hitLabel = "да"
	}

	overall := "PASS"
	if len(failures) > 0 {
		overall = "FAIL"
	}

	fmt.Printf("\n========== SUMMARY ==========\n")
	fmt.Printf("| Параметр | Значение |\n|---|---|\n")
	fmt.Printf("| Натуральный d20 | %d |\n", natural)
	fmt.Printf("| Итог атаки | %d |\n", attackTotal)
	fmt.Printf("| AC | %d |\n", playerAC)
	fmt.Printf("| Попадание | %s |\n", hitLabel)
	fmt.Printf("| Бросок урона | %s |\n", damageParams)
	fmt.Printf("| Урон | %d |\n", damage)
	fmt.Printf("| Оставшиеся HP | %d |\n", hpLeft)
	fmt.Printf("| Вызовов roll_dice | %d |\n", len(rolls))
	fmt.Printf("| Вызовов send_message | %d |\n", len(msgs))
	fmt.Printf("| Время Invoke | %.2fs |\n", elapsed.Seconds())
	fmt.Printf("| Результат | %s |\n", overall)

	fmt.Printf("\n--- Aspect scores ---\n")
	fmt.Printf("Tool usage: %s — %s\n", passFail(toolUsagePass), aspectNote(toolUsagePass,
		"модель сама вызвала roll_dice с корректными аргументами атаки/урона",
		"инструмент не вызван или аргументы не соответствуют правилам"))
	fmt.Printf("Rules reasoning: %s — %s\n", passFail(rulesReasoningPass), aspectNote(rulesReasoningPass,
		"попадание/крит/урон согласованы с d20+4 vs AC 12",
		"ошибка в определении попадания или условного урона"))
	fmt.Printf("Narrative consistency: %s — %s\n", passFail(narrativePass), aspectNote(narrativePass,
		"send_message согласован с механическим исходом",
		"текст противоречит броскам или отсутствует"))

	if len(failures) > 0 {
		fmt.Printf("\n--- Failures ---\n")
		for _, f := range failures {
			fmt.Printf("- %s\n", f)
		}
		t.Fatalf("DungeonMaster goblin crit: FAIL (%d issues)", len(failures))
	}
}

func findDie(args rollArgs, sides, count uint8) (repodices.Die, bool) {
	for _, g := range args.Groups {
		if g.Sides == sides && g.Count == count {
			return g, true
		}
	}
	return repodices.Die{}, false
}

func findDieAnyCount(args rollArgs, sides uint8) (repodices.Die, bool) {
	for _, g := range args.Groups {
		if g.Sides == sides {
			return g, true
		}
	}
	return repodices.Die{}, false
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func aspectNote(ok bool, good, bad string) string {
	if ok {
		return good
	}
	return bad
}
