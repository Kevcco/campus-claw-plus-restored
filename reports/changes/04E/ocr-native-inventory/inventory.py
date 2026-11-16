import json,hashlib,glob,os,subprocess
paths=[]
for root in ['/opt/pdfium','/usr/local/share','/opt/tessdata','/usr/share/licenses']:
 for r,ds,fs in os.walk(root):
  for f in fs:
   p=os.path.join(r,f)
   if any(k in p.lower() for k in ('licen','notice','copying','thirdparty','third_party')): paths.append(p)
paths=sorted(set(paths))
files=[]
for p in paths:
 if os.path.isfile(p):
  b=open(p,'rb').read();files.append(dict(path=p,bytes=len(b),sha256=hashlib.sha256(b).hexdigest()))
versions=subprocess.run(['tesseract','--version'],capture_output=True,text=True,timeout=10)
linked=subprocess.run(['ldd','/usr/local/bin/tesseract'],capture_output=True,text=True,timeout=10)
print(json.dumps(dict(image_scope='research candidate; no production safety claim',tesseract_version_exit=versions.returncode,tesseract_version=versions.stdout+versions.stderr,linked_exit=linked.returncode,linked=linked.stdout+linked.stderr,notice_files=files,apk_packages=open('/opt/apk-packages.txt').read().splitlines()),ensure_ascii=False,indent=2))
