--[[message
青眼一击
入门残局：青眼白龙(3000)已经在场上，对手被打得只剩 500 基本分。
进入战斗阶段攻击恶魔的召唤，穿透防线一击制胜！]]

-- 注意：本引擎 ReloadFieldBegin 会重置整个场地（pduel->clear），
-- 布场指令必须写在它之后。
Debug.ReloadFieldBegin(DUEL_ATTACK_FIRST_TURN + DUEL_SIMPLE_AI, 5)
Debug.SetAIName("残局AI")
Debug.SetPlayerInfo(0, 8000, 0, 1)
Debug.SetPlayerInfo(1, 500, 0, 1)

Debug.AddCard(89631139, 0, 0, LOCATION_MZONE, 2, POS_FACEUP_ATTACK)
Debug.AddCard(70781052, 1, 1, LOCATION_MZONE, 2, POS_FACEUP_ATTACK)
Debug.AddCard(89631139, 1, 1, LOCATION_HAND, 0, POS_FACEDOWN)

Debug.ReloadFieldEnd()
aux.BeginPuzzle()
