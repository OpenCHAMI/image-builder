"""DSL parsing utilities for image-build.

This module provides helpers to load and validate
image-build configuration files written in YAML.
It converts the implicit YAML DSL used by the
image-build tooling into strongly typed Python objects
and performs basic sanity checking.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Union

import yaml


class DSLValidationError(Exception):
    """Raised when the configuration file does not follow the expected DSL."""


@dataclass
class Repo:
    alias: str
    url: str
    gpg: Optional[str] = None
    priority: Optional[Union[int, str]] = None


@dataclass
class Command:
    cmd: str
    loglevel: str = "INFO"


@dataclass
class CopyFile:
    src: str
    dest: str
    opts: List[str] = field(default_factory=list)


@dataclass
class Options:
    layer_type: str
    name: str
    publish_tags: Union[str, List[str], None] = None
    pkg_manager: Optional[str] = None
    parent: str = "scratch"
    publish_local: bool = False
    publish_registry: Optional[str] = None
    registry_opts_push: List[str] = field(default_factory=list)
    registry_opts_pull: List[str] = field(default_factory=list)
    publish_s3: Optional[str] = None
    s3_prefix: str = ""
    s3_bucket: str = "boot-images"
    groups: List[str] = field(default_factory=list)
    playbooks: Optional[Union[str, List[str]]] = None
    inventory: Optional[Union[str, List[str]]] = None
    vars: Dict[str, Any] = field(default_factory=dict)
    ansible_verbosity: int = 0
    labels: Dict[str, str] = field(default_factory=dict)


@dataclass
class ImageDSL:
    options: Options
    repos: List[Repo] = field(default_factory=list)
    modules: Dict[str, List[str]] = field(default_factory=dict)
    packages: List[str] = field(default_factory=list)
    package_groups: List[str] = field(default_factory=list)
    remove_packages: List[str] = field(default_factory=list)
    cmds: List[Command] = field(default_factory=list)
    copyfiles: List[CopyFile] = field(default_factory=list)


def _validate_required(opts: Options) -> None:
    if opts.layer_type not in {"base", "ansible"}:
        raise DSLValidationError("options.layer_type must be 'base' or 'ansible'")

    if not opts.name:
        raise DSLValidationError("options.name is required")

    if opts.layer_type == "base" and not opts.pkg_manager:
        raise DSLValidationError("options.pkg_manager required for base layer")

    if opts.layer_type == "ansible":
        if not opts.groups:
            raise DSLValidationError("options.groups required for ansible layer")
        if not opts.playbooks:
            raise DSLValidationError("options.playbooks required for ansible layer")
        if not opts.inventory:
            raise DSLValidationError("options.inventory required for ansible layer")
        if not isinstance(opts.ansible_verbosity, int) or not (0 <= opts.ansible_verbosity <= 4):
            raise DSLValidationError("options.ansible_verbosity must be 0-4")


def _parse_options(data: Dict[str, Any]) -> Options:
    try:
        return Options(
            layer_type=data["layer_type"],
            name=data["name"],
            publish_tags=data.get("publish_tags"),
            pkg_manager=data.get("pkg_manager"),
            parent=data.get("parent", "scratch"),
            publish_local=data.get("publish_local", False),
            publish_registry=data.get("publish_registry"),
            registry_opts_push=data.get("registry_opts_push", []),
            registry_opts_pull=data.get("registry_opts_pull", []),
            publish_s3=data.get("publish_s3"),
            s3_prefix=data.get("s3_prefix", ""),
            s3_bucket=data.get("s3_bucket", "boot-images"),
            groups=data.get("groups", []),
            playbooks=data.get("playbooks"),
            inventory=data.get("inventory"),
            vars=data.get("vars", {}),
            ansible_verbosity=int(data.get("ansible_verbosity", 0)),
            labels=data.get("labels", {}),
        )
    except KeyError as exc:
        raise DSLValidationError(f"Missing required option: {exc.args[0]}") from exc


def _parse_repos(data: List[Dict[str, Any]]) -> List[Repo]:
    repos: List[Repo] = []
    for item in data:
        if not isinstance(item, dict):
            raise DSLValidationError("repo entries must be dictionaries")
        if "alias" not in item or "url" not in item:
            raise DSLValidationError("repo entry missing 'alias' or 'url'")
        repos.append(
            Repo(
                alias=item["alias"],
                url=item["url"],
                gpg=item.get("gpg"),
                priority=item.get("priority"),
            )
        )
    return repos


def _parse_cmds(data: List[Dict[str, Any]]) -> List[Command]:
    cmds: List[Command] = []
    for item in data:
        if not isinstance(item, dict) or "cmd" not in item:
            raise DSLValidationError("cmd entries must be a mapping with a 'cmd' key")
        loglevel = item.get("loglevel", "INFO")
        cmds.append(Command(cmd=item["cmd"], loglevel=loglevel))
    return cmds


def _parse_copyfiles(data: List[Dict[str, Any]]) -> List[CopyFile]:
    files: List[CopyFile] = []
    for item in data:
        if not isinstance(item, dict) or "src" not in item or "dest" not in item:
            raise DSLValidationError("copyfiles entries need 'src' and 'dest'")
        opts = item.get("opts", [])
        if isinstance(opts, str):
            opts = [opts]
        files.append(CopyFile(src=item["src"], dest=item["dest"], opts=opts))
    return files


def load_dsl(path: str) -> ImageDSL:
    """Load and validate an image-build configuration file."""
    if not os.path.exists(path):
        raise FileNotFoundError(path)

    with open(path, "r", encoding="utf-8") as f:
        raw = yaml.safe_load(f) or {}

    if not isinstance(raw, dict):
        raise DSLValidationError("Top-level YAML structure must be a mapping")

    opt = raw.get("options") or {}
    options = _parse_options(opt)
    _validate_required(options)

    repos = _parse_repos(raw.get("repos", []))
    packages = raw.get("packages", [])
    package_groups = raw.get("package_groups", [])
    remove_packages = raw.get("remove_packages", [])
    modules = raw.get("modules", {})
    cmds = _parse_cmds(raw.get("cmds", []))
    copyfiles = _parse_copyfiles(raw.get("copyfiles", []))

    return ImageDSL(
        options=options,
        repos=repos,
        modules=modules,
        packages=packages,
        package_groups=package_groups,
        remove_packages=remove_packages,
        cmds=cmds,
        copyfiles=copyfiles,
    )


__all__ = [
    "DSLValidationError",
    "Repo",
    "Command",
    "CopyFile",
    "Options",
    "ImageDSL",
    "load_dsl",
]
