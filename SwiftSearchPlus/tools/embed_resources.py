from pathlib import Path
import struct

import argparse

parser = argparse.ArgumentParser(description="Embed the project icon and Windows DPI manifest into a Go-built EXE.")
parser.add_argument("--exe", required=True, help="Input Windows x64 EXE without embedded app resources")
parser.add_argument("--icon", required=True, help="Project ICO file")
parser.add_argument("--output", required=True, help="Output EXE path")
args = parser.parse_args()
SOURCE = Path(args.exe).resolve()
ICO_PATH = Path(args.icon).resolve()
OUTPUT = Path(args.output).resolve()
OUTPUT.parent.mkdir(parents=True, exist_ok=True)
exe = bytearray(SOURCE.read_bytes())
ico = ICO_PATH.read_bytes()

pe = int.from_bytes(exe[0x3c:0x40], 'little')
num = int.from_bytes(exe[pe+6:pe+8], 'little')
opt = pe + 24
szopt = int.from_bytes(exe[pe+20:pe+22], 'little')
sect = opt + szopt
file_align = int.from_bytes(exe[opt+36:opt+40], 'little')
section_align = int.from_bytes(exe[opt+32:opt+36], 'little')
size_headers = int.from_bytes(exe[opt+60:opt+64], 'little')
assert sect + num * 40 + 40 <= size_headers, 'No room for a new PE section header'

def align(n, a):
    return (n + a - 1) // a * a

def pack_dir(named, ids):
    return struct.pack('<IIHHHH', 0, 0, 0, 0, named, ids)

def pack_entry(id_value, offset, is_dir=False):
    return struct.pack('<II', id_value, offset | (0x80000000 if is_dir else 0))

reserved, icon_type, count = struct.unpack_from('<HHH', ico, 0)
assert reserved == 0 and icon_type == 1 and count > 0
icons = []
for i in range(count):
    eoff = 6 + i * 16
    width, height, colors, rsv, planes, bpp, size, data_off = struct.unpack_from('<BBBBHHII', ico, eoff)
    payload = ico[data_off:data_off + size]
    if len(payload) != size:
        raise ValueError('Truncated ICO image')
    icons.append((ico[eoff:eoff+12], payload))

group = bytearray(struct.pack('<HHH', 0, 1, count))
for i, (meta, _payload) in enumerate(icons, 1):
    group += meta + struct.pack('<H', i)

resource = bytearray()
def reserve(n, alignment=4):
    while len(resource) % alignment:
        resource.append(0)
    pos = len(resource)
    resource.extend(b'\0' * n)
    return pos

root = reserve(16 + 3 * 8)
type_icons = reserve(16 + len(icons) * 8)
type_group = reserve(16 + 8)
type_manifest = reserve(16 + 8)
icon_nodes = [reserve(16 + 8) for _ in icons]
group_node = reserve(16 + 8)
manifest_node = reserve(16 + 8)
data_nodes = [reserve(16) for _ in icons]
group_data_node = reserve(16)
manifest_data_node = reserve(16)
image_offsets = []
for _meta, payload in icons:
    off = reserve(len(payload), 4)
    resource[off:off+len(payload)] = payload
    image_offsets.append(off)
group_offset = reserve(len(group), 4)
resource[group_offset:group_offset+len(group)] = group
manifest = b'''<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">\n  <dependency><dependentAssembly><assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*" /></dependentAssembly></dependency>\n  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3"><security><requestedPrivileges><requestedExecutionLevel level="asInvoker" uiAccess="false" /></requestedPrivileges></security></trustInfo>\n  <application xmlns="urn:schemas-microsoft-com:asm.v3"><windowsSettings><dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware><dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2,PerMonitor</dpiAwareness></windowsSettings></application>\n</assembly>'''
manifest_offset = reserve(len(manifest), 4)
resource[manifest_offset:manifest_offset+len(manifest)] = manifest

resource[root:root+16] = pack_dir(0, 3)
resource[root+16:root+24] = pack_entry(3, type_icons, True)  # RT_ICON
resource[root+24:root+32] = pack_entry(14, type_group, True)  # RT_GROUP_ICON
resource[root+32:root+40] = pack_entry(24, type_manifest, True)  # RT_MANIFEST
resource[type_icons:type_icons+16] = pack_dir(0, len(icons))
for i, node in enumerate(icon_nodes, 1):
    resource[type_icons+16+(i-1)*8:type_icons+24+(i-1)*8] = pack_entry(i, node, True)
resource[type_group:type_group+16] = pack_dir(0, 1)
resource[type_group+16:type_group+24] = pack_entry(1, group_node, True)
resource[type_manifest:type_manifest+16] = pack_dir(0, 1)
resource[type_manifest+16:type_manifest+24] = pack_entry(1, manifest_node, True)
for node, data_node in zip(icon_nodes, data_nodes):
    resource[node:node+16] = pack_dir(0, 1)
    resource[node+16:node+24] = pack_entry(0, data_node)
resource[group_node:group_node+16] = pack_dir(0, 1)
resource[group_node+16:group_node+24] = pack_entry(0, group_data_node)
resource[manifest_node:manifest_node+16] = pack_dir(0, 1)
resource[manifest_node+16:manifest_node+24] = pack_entry(0, manifest_data_node)

old_size_image = int.from_bytes(exe[opt+56:opt+60], 'little')
section_rva = align(old_size_image, section_align)
raw_ptr = align(len(exe), file_align)
raw_size = align(len(resource), file_align)
for data_node, offset, (_meta, payload) in zip(data_nodes, image_offsets, icons):
    resource[data_node:data_node+16] = struct.pack('<IIII', section_rva + offset, len(payload), 0, 0)
resource[group_data_node:group_data_node+16] = struct.pack('<IIII', section_rva + group_offset, len(group), 0, 0)
resource[manifest_data_node:manifest_data_node+16] = struct.pack('<IIII', section_rva + manifest_offset, len(manifest), 65001, 0)

if len(exe) < raw_ptr:
    exe.extend(b'\0' * (raw_ptr - len(exe)))
exe.extend(resource)
exe.extend(b'\0' * (raw_ptr + raw_size - len(exe)))

header = bytearray(40)
header[:8] = b'.rsrc\0\0\0'
struct.pack_into('<IIIIIIHHI', header, 8, len(resource), section_rva, raw_size, raw_ptr, 0, 0, 0, 0, 0x40000040)
new_section_header = sect + num * 40
exe[new_section_header:new_section_header+40] = header
struct.pack_into('<H', exe, pe+6, num+1)
struct.pack_into('<I', exe, opt+56, align(section_rva + len(resource), section_align))
struct.pack_into('<I', exe, opt+8, int.from_bytes(exe[opt+8:opt+12], 'little') + raw_size)
# IMAGE_DIRECTORY_ENTRY_RESOURCE is data directory #2 in PE32+.
struct.pack_into('<II', exe, opt+112+2*8, section_rva, len(resource))
OUTPUT.write_bytes(exe)
print(f'Wrote {OUTPUT} ({len(exe):,} bytes), embedded {count} icon sizes and common-controls/DPI manifest')
