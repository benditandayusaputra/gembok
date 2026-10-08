import sys
import xml.etree.ElementTree as ET
from pathlib import Path

def main() -> int:
    total = gagal = lewat = 0
    for name in sys.argv[1:]:
        p = Path(name)
        if not p.exists():
            print(f"{name}: berkas hasil tidak ada")
            return 1
        for tc in ET.parse(p).getroot().iter("testcase"):
            total += 1
            if tc.find("failure") is not None or tc.find("error") is not None:
                gagal += 1
                print(f"GAGAL: {tc.get('name')}")
            elif tc.find("skipped") is not None:
                lewat += 1
    print(f"{total - gagal - lewat} lolos, {gagal} gagal, {lewat} dilewati")
    return 1 if gagal or total == lewat else 0

if __name__ == "__main__":
    sys.exit(main())
