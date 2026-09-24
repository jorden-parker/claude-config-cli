import re, json, os, subprocess, sys
here = os.path.dirname(os.path.abspath(__file__))
os.chdir(here)
subprocess.run([sys.executable, 'extract.py'], check=True, stdout=subprocess.DEVNULL)
entries = json.load(open('entries.json'))
src = open('ref.md').read()

def deprecated(key):
    m = re.search(r'### `'+re.escape(key)+r'`\n(.*?)(?=\n### |\n## )', src, re.S)
    body = m.group(1) if m else ''
    w = re.search(r'<Warning>(.*?)</Warning>', body, re.S)
    if w and re.search(r'[Rr]emoved|[Dd]eprecated|no effect', w.group(1)):
        return re.sub(r'\s+',' ', re.sub(r'\[([^\]]+)\]\([^)]*\)', r'\1', w.group(1))).strip()
    return ''

def clean(s):
    s = re.sub(r'\[([^\]]+)\]\([^)]*\)', r'\1', s)
    s = re.sub(r'\*\*([^*]+)\*\*', r'\1', s)
    return s.strip()

MODEL_ALIASES = ["default","best","fable","sonnet","opus","haiku","sonnet[1m]","opus[1m]","fable[1m]","opusplan"]
HOOK_EVENTS = ["SessionStart","Setup","InstructionsLoaded","UserPromptSubmit","UserPromptExpansion","MessageDisplay","PreToolUse","PermissionRequest","PostToolUse","PostToolUseFailure","PostToolBatch","PermissionDenied","Notification","SubagentStart","SubagentStop","TaskCreated","TaskCompleted","Stop","StopFailure","TeammateIdle","ConfigChange","CwdChanged","DirectoryAdded","FileChanged","WorktreeCreate","WorktreeRemove","PreCompact","PostCompact","PreModelSwitch","PostModelSwitch","SessionEnd","Elicitation","ElicitationResult"]

overrides = {
 'model': dict(kind='enum-or-string', options=MODEL_ALIASES, hint='alias or full model ID such as claude-opus-5-5'),
 'advisorModel': dict(kind='enum-or-string', options=["fable","opus","sonnet"], hint='alias or full model ID'),
 'teammateDefaultModel': dict(kind='enum-or-string', options=MODEL_ALIASES, hint='alias or full model ID'),
 'availableModels': dict(kind='array', suggestions=MODEL_ALIASES),
 'fallbackModel': dict(kind='array', suggestions=MODEL_ALIASES),
 'outputStyle': dict(kind='enum-or-string', options=["Default","Proactive","Concise","Explanatory","Learning"], hint='built-in name (case-sensitive) or a custom style name'),
 'theme': dict(kind='enum-or-string', options=["auto","dark","light","dark-daltonized","light-daltonized","dark-ansi","light-ansi"], hint='or custom:<slug>'),
 'timeFormat': dict(kind='enum-or-string', options=["auto","12-hour","24-hour","24-hour-utc"], hint='or a strftime pattern such as %H:%M'),
 'strictPluginOnlyCustomization': dict(kind='multiselect', options=["skills","agents","hooks","mcp"], hint='true locks all four; array locks the named kinds'),
 'hooks': dict(kind='json', suggestions=HOOK_EVENTS),
 'env': dict(kind='map'),
 'enabledPlugins': dict(kind='map-bool'),
 'modelOverrides': dict(kind='map'),
 'vimInsertModeRemaps': dict(kind='map'),
 'forceLoginOrgUUID': dict(kind='array'),
 'permissions.defaultMode': dict(kind='enum'),
 'taskOutputMaxChars': dict(kind='number'),
 'disableDesktopLocalSessions': dict(kind='bool'), 'disableBrowserExternalNavigation': dict(kind='bool'), 'disableMobileSimulatorTools': dict(kind='bool'),
 'browserExternalPageTools': dict(kind='enum', options=["disabled"]),
}
skip = {'strictPluginOnlyCustomization.skills','strictPluginOnlyCustomization.agents','strictPluginOnlyCustomization.hooks','strictPluginOnlyCustomization.mcp'}

def classify(e):
    t = e['type']; opts=[o['value'].strip('"') for o in e['options']]
    if t.startswith('Boolean'): return 'bool', []
    m = re.match(r'the string `"([^"]+)"`', t)
    if m: return 'enum', [m.group(1)]
    if e['options'] and all(o['value'].startswith('"') for o in e['options']):
        return 'enum', opts
    if t.startswith('string, one of') or t.startswith('string, `"'):
        vals = list(dict.fromkeys(re.findall(r'`"([^"]+)"`', t)))
        if vals: return 'enum', vals
    if t.startswith('number') or t.startswith('integer'): return 'number', []
    if t.startswith('array of objects') or t.startswith('array of marketplace source'): return 'json', []
    if t.startswith('array'): return 'array', []
    if t.startswith('object'): return 'json', []
    return 'string', []

keys = {e['key'] for e in entries}
out=[]
for e in entries:
    if e['key'] in skip: continue
    kind, opts = classify(e)
    children = [k for k in keys if k.startswith(e['key']+'.')]
    item = dict(key=e['key'], section=e['section'], scope=clean(e['scope']), type=clean(e['type']), default=clean(e['default']),
                desc=clean(e['desc']), kind=kind, options=opts,
                optionHelp={o['value'].strip('"'):clean(o['help']) for o in e['options'] if o['help']},
                example=e['example'], deprecated=deprecated(e['key']), hint='', suggestions=[])
    if children and kind=='json': item['kind']='group'
    item.update(overrides.get(e['key'], {}))
    item['global'] = e['section']=='Global config settings'
    sc = clean(e['scope'])
    head = sc.split('.')[0].strip()
    cls = 'any'
    if head.startswith('Managed'): cls='managed'
    elif head.startswith('User, local, or managed'): cls='user-local-managed'
    elif head.startswith('User or managed'): cls='user-managed'
    elif head.startswith('Global config'): cls='global'
    item['scopeClass']=cls
    item['scopeNote']=sc[len(head)+1:].strip() if '.' in sc else ''
    item['scope']=head
    out.append(item)

# env vars
env=[]
for l in open('env.md'):
    m = re.match(r'\| `([A-Z0-9_]+)`\s+\| (.*?)\s+\|', l)
    if m: env.append(dict(name=m.group(1), purpose=clean(m.group(2))[:300]))
os.remove('entries.json')
json.dump(dict(generatedFrom="https://code.claude.com/docs/en/settings-reference.md", generatedOn="2026-09-24", settings=out, envVars=env, hookEvents=HOOK_EVENTS, modelAliases=MODEL_ALIASES), open(os.path.join(here, '..', 'internal', 'schema', 'schema.json'),'w'), indent=1)
from collections import Counter
print(len(out), Counter(i['kind'] for i in out), len(env))
print([i['key'] for i in out if i['deprecated']])
print([i['key'] for i in out if i['kind']=='group'])
