# -*- mode: python ; coding: utf-8 -*-
import os


a = Analysis(
    [os.path.join(SPECPATH, 'src', 'agent', 'main.py')],
    pathex=[os.path.join(SPECPATH, 'src'), os.path.join(SPECPATH, 'src', 'agent')],
    binaries=[],
    datas=[],
    hiddenimports=['agent.agent_pb2', 'agent.agent_pb2_grpc', 'agent_pb2', 'agent_pb2_grpc', 'grpc', 'google'],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
    optimize=0,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name='flowpartner-agent',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)