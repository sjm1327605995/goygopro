#!/usr/bin/env python3
"""Fetch the pinned external Lua rules. Never overwrite existing local scripts."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


def main():
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1], help="项目或分发包根目录")
    args = parser.parse_args()
    root = args.root.expanduser().resolve()
    if not root.is_dir():
        parser.error(f"根目录不存在：{root}")
    manifest = json.loads((Path(__file__).resolve().parents[1] / "resources/scripts.json").read_text(encoding="utf-8"))
    if not re.fullmatch(r"[0-9a-f]{40}", manifest["commit"]) or manifest["directory"] != "script":
        parser.error("脚本清单必须使用完整提交 SHA，目标目录必须为 script")
    destination = root / "script"
    if destination.exists() and (not destination.is_dir() or any(destination.iterdir())):
        print(f"未覆盖已有资源：{destination}\n如现有脚本可用，可继续使用。需要更换版本时请先自行备份；获取脚本的步骤见 docs/setup-and-troubleshooting.md。", file=sys.stderr)
        return 1
    def git(*arguments):
        subprocess.run(["git", "-C", str(destination), *arguments], check=True)
    try:
        subprocess.run(["git", "init", "--quiet", str(destination)], check=True)
        git("remote", "add", "origin", manifest["repository"])
        git("fetch", "--depth", "1", "origin", manifest["commit"])
        git("checkout", "--quiet", "--detach", "FETCH_HEAD")
        for name in ["constant.lua", "utility.lua", "procedure.lua"]:
            if not (destination / name).is_file():
                raise RuntimeError(f"上游脚本缺少 {name}")
        revision = subprocess.check_output(["git", "-C", str(destination), "rev-parse", "HEAD"], text=True).strip()
        if revision != manifest["commit"]:
            raise RuntimeError("获取的脚本版本与清单不一致")
        print(f"脚本已准备：{destination}\n版本：{revision}\n请继续运行 python tools/check_resources.py --root \"{root}\"")
        return 0
    except (OSError, subprocess.CalledProcessError, RuntimeError) as error:
        print(f"获取失败：{error}\n未删除或覆盖原有文件。若留下新建的未完成目录，请检查网络后自行处理该目录，或按文档手动完成 Git 获取。", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
