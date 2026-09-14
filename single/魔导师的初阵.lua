--[[message
魔导师的初阵
进阶残局：手牌里有黑魔导(2500)。
通常召唤它，然后进入战斗阶段直接攻击，把对手的 2500 基本分打空！]]

-- 注意：本引擎 ReloadFieldBegin 会重置整个场地（pduel->clear），
-- 布场指令必须写在它之后。
Debug.ReloadFieldBegin(DUEL_ATTACK_FIRST_TURN + DUEL_SIMPLE_AI, 5)
Debug.SetAIName("残局AI")
Debug.SetPlayerInfo(0, 8000, 0, 1)
Debug.SetPlayerInfo(1, 2500, 0, 1)

Debug.AddCard(46986414, 0, 0, LOCATION_HAND, 0, POS_FACEDOWN)
Debug.AddCard(89631139, 0, 0, LOCATION_HAND, 1, POS_FACEDOWN)
Debug.AddCard(89631139, 0, 0, LOCATION_DECK, 0, POS_FACEDOWN)
Debug.AddCard(89631139, 0, 0, LOCATION_DECK, 0, POS_FACEDOWN)
Debug.AddCard(89631139, 0, 0, LOCATION_DECK, 0, POS_FACEDOWN)

Debug.ReloadFieldEnd()
aux.BeginPuzzle()
