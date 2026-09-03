-- +goose Up
-- 一条便签被摆进了「问题识别」四格板的哪一格：谁 / 需要什么 / 为什么。
-- 空 = 还在「我们看到的证据」那一堆里，没被判过。
--
-- 🚨 为什么不复用 cluster。cluster 是**便签板上的归堆**（"这几张是一回事"），
-- reframe_slot 是**问题陈述里的角色**（"这条是在说谁"）。它们是关于同一张纸的
-- 两句不同的判断，而且都是她自己做的——共用一列意味着她在问题识别里摆一下，
-- 就把自己在板上归的堆悄悄擦掉了。悄悄擦掉她的判断，是这个产品最不该做的事。
--
-- 🚨 NOT NULL DEFAULT ''：和 cluster 同一种形状。空串而不是 NULL，是为了让
-- 「没摆过」和「摆回证据里」是同一个状态——她把纸拖回去，就是收回那句判断。
ALTER TABLE pbl_note
  ADD COLUMN reframe_slot text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pbl_note DROP COLUMN reframe_slot;
