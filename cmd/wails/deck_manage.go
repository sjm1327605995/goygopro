package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 卡组/分类管理（原版 wDeckManage，deck_con.cpp:423-620 的 BUTTON_DM_OK 各分支）。
// 目录结构与 DeckManager 一致：./deck/<分类>/<名>.ydk，根目录 = 未分类。
// 分类名只允许单段（拒绝 '/' 与穿越写法），卡组名沿用 safeDeckPath 的校验。

// safeCategoryName 校验分类名：非空、单段、不含路径分隔符与穿越段。
func safeCategoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("分类名不能为空")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
		return "", fmt.Errorf("非法分类名：%q", name)
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("非法分类名：%q", name)
	}
	return name, nil
}

func (a *App) categoryPath(name string) (string, error) {
	safe, err := safeCategoryName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(a.deckDir, safe), nil
}

// CreateDeckCategory 新建分类目录（DeckManager::CreateCategory）。已存在报错。
func (a *App) CreateDeckCategory(name string) error {
	dir, err := a.categoryPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("分类已存在：%s", name)
	}
	return os.MkdirAll(dir, 0o755)
}

// RenameDeckCategory 重命名分类目录（DeckManager::RenameCategory）。目标已存在报错。
func (a *App) RenameDeckCategory(oldName string, newName string) error {
	oldDir, err := a.categoryPath(oldName)
	if err != nil {
		return err
	}
	newDir, err := a.categoryPath(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldDir); err != nil {
		return fmt.Errorf("分类不存在：%s", oldName)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("分类已存在：%s", newName)
	}
	return os.Rename(oldDir, newDir)
}

// DeleteDeckCategory 删除分类目录（DeckManager::DeleteCategory，连内容一起删）。
func (a *App) DeleteDeckCategory(name string) error {
	dir, err := a.categoryPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("分类不存在：%s", name)
	}
	return os.RemoveAll(dir)
}

// resolveDeckTarget 解析卡组重命名/复制目标：新名不含 '/' 时沿用旧名的分类
// （原版 BUTTON_RENAME_DECK 只在当前分类内改名）。目标已存在则报错
// （原版 IsFileExists → SysString 1475）。
func (a *App) resolveDeckTarget(oldName string, newName string) (oldPath, newPath string, err error) {
	oldPath, err = safeDeckPath(a.deckDir, oldName)
	if err != nil {
		return "", "", err
	}
	trimmed := strings.TrimSpace(newName)
	if !strings.Contains(trimmed, "/") {
		if idx := strings.LastIndex(filepath.ToSlash(oldName), "/"); idx >= 0 {
			trimmed = filepath.ToSlash(oldName)[:idx+1] + trimmed
		}
	}
	newPath, err = safeDeckPath(a.deckDir, trimmed)
	if err != nil {
		return "", "", err
	}
	if _, statErr := os.Stat(oldPath); statErr != nil {
		return "", "", fmt.Errorf("卡组不存在：%s", oldName)
	}
	if oldPath != newPath {
		if _, statErr := os.Stat(newPath); statErr == nil {
			return "", "", fmt.Errorf("卡组已存在：%s", trimmed)
		}
	}
	return oldPath, newPath, nil
}

// RenameDeck 重命名卡组（BUTTON_RENAME_DECK）。
func (a *App) RenameDeck(oldName string, newName string) error {
	oldPath, newPath, err := a.resolveDeckTarget(oldName, newName)
	if err != nil {
		return err
	}
	if oldPath == newPath {
		return nil
	}
	return os.Rename(oldPath, newPath)
}

// CopyDeck 把卡组复制到其他分类，保持卡組名不变（BUTTON_COPY_DECK：原版
// 从 cbDMCategory 选目标分类，以同名另存一份）。category 为空串 = 未分类
// （根目录）。目标已存在报错。
func (a *App) CopyDeck(name string, category string) error {
	oldPath, err := safeDeckPath(a.deckDir, name)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(oldPath); statErr != nil {
		return fmt.Errorf("卡组不存在：%s", name)
	}
	base := filepath.ToSlash(name)
	base = base[strings.LastIndex(base, "/")+1:]
	cat := strings.TrimSpace(category)
	target := base
	if cat != "" {
		if _, err := a.categoryPath(cat); err != nil {
			return err
		}
		target = cat + "/" + base
	}
	newPath, err := safeDeckPath(a.deckDir, target)
	if err != nil {
		return err
	}
	if oldPath == newPath {
		return errors.New("目标分类与原分类相同")
	}
	if _, statErr := os.Stat(newPath); statErr == nil {
		return fmt.Errorf("卡组已存在：%s", target)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(oldPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(newPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// MoveDeck 把卡组移动到其他分类（BUTTON_MOVE_DECK）。category 为空串 =
// 未分类（根目录）。目标已存在报错。
func (a *App) MoveDeck(name string, category string) error {
	oldPath, err := safeDeckPath(a.deckDir, name)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(oldPath); statErr != nil {
		return fmt.Errorf("卡组不存在：%s", name)
	}
	base := filepath.ToSlash(name)
	base = base[strings.LastIndex(base, "/")+1:]
	cat := strings.TrimSpace(category)
	target := base
	if cat != "" {
		target = cat + "/" + base
	}
	newPath, err := safeDeckPath(a.deckDir, target)
	if err != nil {
		return err
	}
	if oldPath == newPath {
		return nil
	}
	if _, statErr := os.Stat(newPath); statErr == nil {
		return fmt.Errorf("卡组已存在：%s", base)
	}
	if strings.TrimSpace(category) != "" {
		if _, err := a.categoryPath(cat); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
	}
	return os.Rename(oldPath, newPath)
}
