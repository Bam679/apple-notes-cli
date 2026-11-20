// Static Notes.app bridge. Never interpolate input into this file: all input
// arrives as decoded JSON in `a` (see osa.Prelude).
const N = Application('Notes');

function iso(d) { return d ? d.getTime() : null; }

// folderIndex maps folder id -> {id, name, path, account, parent}.
function folderIndex() {
  const idx = {};
  const accs = N.accounts;
  const accNames = accs.name();
  const accIds = accs.id();
  for (let k = 0; k < accNames.length; k++) {
    const a = accs.byId(accIds[k]);
    const ids = a.folders.id();
    const names = a.folders.name();
    const parents = {};
    const live = [];
    for (let i = 0; i < ids.length; i++) {
      let pid;
      try { pid = a.folders.byId(ids[i]).container().id(); } catch (e) { continue; } // vanished while listing
      parents[ids[i]] = pid === accIds[k] ? null : pid;
      idx[ids[i]] = { id: ids[i], name: names[i], account: accNames[k], account_id: accIds[k] };
      live.push(ids[i]);
    }
    const pathOf = function (id, depth) {
      const f = idx[id];
      if (f.path) return f.path;
      const p = parents[id];
      f.path = (p && idx[p] && depth < 32) ? pathOf(p, depth + 1) + '/' + f.name : f.account + '/' + f.name;
      return f.path;
    };
    live.forEach(function (id) { pathOf(id, 0); idx[id].parent_id = parents[id]; });
  }
  return idx;
}

function folderSpec(id) {
  const f = N.folders.byId(id);
  if (!f.exists()) fail('not_found', 'no folder with id ' + id);
  return f;
}

function noteSpec(id) {
  const n = N.notes.byId(id);
  if (!n.exists()) fail('not_found', 'no note with id ' + id);
  return n;
}

function noteOut(n, idx, withBody) {
  const folderId = n.container().id();
  const f = idx[folderId] || { path: '', name: '' };
  const locked = n.passwordProtected();
  const o = {
    id: n.id(), title: n.name(), folder: f.path, folder_id: folderId,
    created_ms: iso(n.creationDate()), modified_ms: iso(n.modificationDate()),
    locked: locked, attachments: n.attachments.length,
  };
  if (withBody && !locked) {
    o.text = n.plaintext();
    o.html = n.body();
  }
  return o;
}

const OPS = {
  accounts: function () {
    const ids = N.accounts.id(), names = N.accounts.name();
    return ids.map(function (id, i) { return { id: id, name: names[i] }; });
  },
  folders: function () {
    const idx = folderIndex();
    return Object.keys(idx).map(function (id) {
      const f = idx[id];
      let count = 0;
      try { count = N.folders.byId(id).notes.length; } catch (e) { count = 0; }
      return { id: f.id, name: f.name, path: f.path, account: f.account, parent_id: f.parent_id || '', notes: count };
    });
  },
  notes: function (a) {
    const idx = folderIndex();
    const ids = (a.folder_ids && a.folder_ids.length) ? a.folder_ids : Object.keys(idx);
    const out = [];
    ids.forEach(function (fid) {
      if (!idx[fid]) fail('not_found', 'no folder with id ' + fid);
      const ns = N.folders.byId(fid).notes;
      const nids = ns.id();
      if (!nids.length) return;
      const names = ns.name(), mod = ns.modificationDate(), cre = ns.creationDate(), pw = ns.passwordProtected();
      let texts = null;
      if (a.with_text) {
        try { texts = ns.plaintext(); } catch (e) {
          texts = nids.map(function (id, i) { return pw[i] ? '' : N.notes.byId(id).plaintext(); });
        }
      }
      for (let i = 0; i < nids.length; i++) {
        const o = { id: nids[i], title: names[i], folder: idx[fid].path, folder_id: fid,
          created_ms: iso(cre[i]), modified_ms: iso(mod[i]), locked: pw[i] };
        if (texts && !pw[i]) o.text = texts[i];
        out.push(o);
      }
    });
    return out;
  },
  get: function (a) {
    return noteOut(noteSpec(a.id), folderIndex(), true);
  },
  add: function (a) {
    const f = folderSpec(a.folder_id);
    const n = N.make({ new: 'note', at: f, withProperties: { body: a.html } });
    return noteOut(n, folderIndex(), true);
  },
  set_body: function (a) {
    const n = noteSpec(a.id);
    if (n.passwordProtected()) fail('conflict', 'note is locked');
    n.body = a.append ? n.body() + a.html : a.html;
    return noteOut(n, folderIndex(), true);
  },
  move: function (a) {
    const n = noteSpec(a.id);
    N.move(n, { to: folderSpec(a.folder_id) });
    return noteOut(noteSpec(a.id), folderIndex(), false);
  },
  delete: function (a) {
    const n = noteSpec(a.id);
    N.delete(n);
    if (a.purge) {
      // Second delete removes it from "Recently Deleted" (test cleanup only).
      const again = N.notes.byId(a.id);
      if (again.exists()) N.delete(again);
    }
    return { id: a.id };
  },
  folder_create: function (a) {
    let parent;
    if (a.parent_id) parent = folderSpec(a.parent_id);
    else {
      const acc = N.accounts.byId(a.account_id);
      if (!acc.exists()) fail('not_found', 'no account with id ' + a.account_id);
      parent = acc;
    }
    const f = N.make({ new: 'folder', at: parent, withProperties: { name: a.name } });
    const idx = folderIndex();
    const out = idx[f.id()];
    return { id: out.id, name: out.name, path: out.path, account: out.account, parent_id: out.parent_id || '', notes: 0 };
  },
  folder_delete: function (a) {
    const f = folderSpec(a.id);
    if (!a.force && (f.notes.length > 0 || f.folders.length > 0)) fail('conflict', 'folder is not empty');
    if (a.force) f.notes().forEach(function (n) { const id = n.id(); N.delete(n); const again = N.notes.byId(id); if (again.exists()) N.delete(again); });
    N.delete(f);
    return { id: a.id };
  },
};
