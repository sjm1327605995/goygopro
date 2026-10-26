#!/usr/bin/env python3
"""Read-only runtime resource audit; standard library only, no engine execution."""
import argparse
from contextlib import closing
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import sqlite3
import struct
import subprocess
import sys


def audit(root: Path) -> dict:
    checks = []

    def add(name, status, detail, **values):
        checks.append(dict(name=name, status=status, detail=detail, **values))

    def required_file(relative):
        path = root / relative
        present = path.is_file() and path.stat().st_size > 0
        values = dict(path=str(path))
        if present:
            digest = hashlib.sha256()
            with path.open("rb") as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(block)
            values.update(size=path.stat().st_size, sha256=digest.hexdigest())
        add(relative, "OK" if present else "ERROR", "存在且非空" if present else "缺失或为空", **values)
        return present

    database = root / "cards.cdb"
    if required_file("cards.cdb"):
        try:
            # immutable avoids creating journal/SHM files. Inspect a closed/static
            # card database, not an actively edited SQLite WAL snapshot.
            with closing(sqlite3.connect(database.as_uri() + "?mode=ro&immutable=1", uri=True)) as connection:
                expected = {"datas": set("id ot alias setcode type atk def level race attribute category".split()),
                            "texts": {"id", "name", "desc", *(f"str{i}" for i in range(1, 17))}}
                for table, columns in expected.items():
                    actual = {row[1] for row in connection.execute(f"PRAGMA table_info({table})")}
                    if columns - actual:
                        raise ValueError(f"{table} 缺少字段：{', '.join(sorted(columns - actual))}")
                datas = connection.execute("SELECT count(*) FROM datas").fetchone()[0]
                texts = connection.execute("SELECT count(*) FROM texts").fetchone()[0]
                matching = connection.execute("SELECT count(*) FROM datas JOIN texts ON datas.id=texts.id").fetchone()[0]
                if not datas or not texts or not matching:
                    raise ValueError("datas/texts 没有可关联的卡片数据")
            add("database_schema", "OK", "datas/texts 可读且存在关联卡片", datas=datas, texts=texts, matching_ids=matching)
        except (sqlite3.Error, ValueError) as error:
            add("database_schema", "ERROR", str(error))

    for relative in ["script/constant.lua", "script/utility.lua", "script/procedure.lua", "frontend/dist/index.html", "frontend/dist/textures/cover.jpg"]:
        required_file(relative)
    scripts = sum(1 for p in (root / "script").glob("c*.lua") if p.is_file() and re.fullmatch(r"c\d+\.lua", p.name))
    add("card_scripts", "OK" if scripts else "ERROR", f"发现 {scripts} 个 c<卡号>.lua；存在不代表与引擎版本兼容", count=scripts)

    singles = [p for p in (root / "single").glob("*") if p.is_file() and p.suffix == ".lua"]
    decks = [p for p in (root / "deck").rglob("*") if p.is_file() and p.suffix == ".ydk"]
    add("single_list", "OK" if singles else "WARN", f"single/ 直接子目录中有 {len(singles)} 个小写 .lua 文件", files=[p.name for p in singles])
    add("deck_list", "OK" if decks else "WARN", f"deck/ 中有 {len(decks)} 个 .ydk；空目录不会生成示例卡组", files=[p.relative_to(root).as_posix() for p in decks])
    add("strings.conf", "OK" if (root / "strings.conf").is_file() else "WARN", "系统字符串、系列名和指示物名称资源")

    library_name = {"Windows": "ocgcore.dll", "Darwin": "libocgcore.dylib"}.get(platform.system(), "libocgcore.so")
    if required_file(library_name) and platform.system() == "Windows":
        try:
            with (root / library_name).open("rb") as stream:
                header = stream.read(64)
                if len(header) < 64 or header[:2] != b"MZ":
                    raise ValueError("不是有效的 PE 动态库头")
                stream.seek(struct.unpack_from("<I", header, 60)[0])
                signature = stream.read(6)
                if len(signature) != 6 or signature[:4] != b"PE\0\0":
                    raise ValueError("PE 签名无效")
                machine = struct.unpack_from("<H", signature, 4)[0]
                expected = {"amd64": 0x8664, "x86_64": 0x8664, "arm64": 0xAA64, "aarch64": 0xAA64}.get(platform.machine().lower())
                add("engine_architecture", "OK" if expected is None or machine == expected else "WARN", "仅检查 PE 架构；不能确认 ABI、依赖 DLL 或 Lua API 兼容性", machine=hex(machine), host=platform.machine())
        except (OSError, ValueError, struct.error) as error:
            add("engine_architecture", "ERROR", str(error))

    roots = [root]
    if os.environ.get("YGOPRO_PICS_DIR"):
        roots.insert(0, Path(os.environ["YGOPRO_PICS_DIR"]).expanduser().resolve())
    picture_dirs = [base / relative for base in roots for relative in ["pics", "expansions/pics", "YGOPro/pics"]]
    pictures = sum(1 for directory in picture_dirs if directory.is_dir() for p in directory.iterdir() if p.is_file() and p.suffix.lower() in {".jpg", ".png", ".webp"})
    add("card_pictures", "OK" if pictures else "WARN", f"标准卡图目录找到 {pictures} 张图片。缺图影响显示，不会让单机列表为空；CDN 未在本工具中探测", directories=[str(p) for p in picture_dirs])
    fields = sum(1 for base in roots for relative in ["pics/field", "expansions/pics/field"] if (base / relative).is_dir() for p in (base / relative).iterdir() if p.is_file() and p.suffix.lower() in {".png", ".jpg"})
    add("field_pictures", "OK" if fields else "WARN", f"发现 {fields} 张场地魔法图；缺图时保留基础场地，不从卡片正面图自动推导场地图")
    add("wails_bridge", "INFO", "本工具不能验证应用是否连接 Wails。纯 Vite 页面使用 Mock，必须另外按文档核对启动方式。")
    try:
        revision = subprocess.check_output(["git", "-C", str(root), "rev-parse", "--short", "HEAD"], stderr=subprocess.DEVNULL, text=True).strip()
    except (OSError, subprocess.CalledProcessError):
        revision = None
    return dict(root=str(root), revision=revision, platform=platform.platform(), checks=checks,
                errors=sum(c["status"] == "ERROR" for c in checks), warnings=sum(c["status"] == "WARN" for c in checks))


def main():
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1], help="资源根目录，不是 bin/ 或 frontend/ 目录")
    parser.add_argument("--json", action="store_true", help="输出可保存的 JSON 报告")
    parser.add_argument("--output", type=Path, help="额外保存 UTF-8 JSON 报告；仅允许 .json 扩展名")
    args = parser.parse_args()
    root = args.root.expanduser().resolve()
    if not root.is_dir():
        parser.error(f"目录不存在：{root}")
    try:
        report = audit(root)
    except OSError as error:
        parser.exit(2, f"无法读取资源：{error}\n")
    if args.output:
        if args.output.suffix.lower() != ".json":
            parser.error("报告文件必须使用 .json 扩展名")
        try:
            args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        except OSError as error:
            parser.exit(2, f"无法保存报告：{error}\n")
    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print(f"Resource root: {root}\nGit: {report['revision'] or 'not a checkout'}\n")
        for check in report["checks"]:
            print(f"[{check['status']}] {check['name']}: {check['detail']}")
        print(f"\nErrors: {report['errors']}; warnings: {report['warnings']}")
    return 1 if report["errors"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
