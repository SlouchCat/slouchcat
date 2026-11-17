/* M-35 «Einladung annehmen» (#/invite/:id): the account with its main profile, and at the same moment its user in
   Seerr with the same login. The password lives only in ui.draft.invite and is dropped after submit. Prefix va-. */
'use strict';

function vaSeerrOn() { return !!(S.settings.wishes.provision && S.seerr.configured && S.seerr.enabled); }
function vaLimitText(inv) {
  const l = inv.limit || { mode: 'default' };
  const c = l.mode === 'unlimited' ? null : l.mode === 'custom' ? l : S.settings.wishes.limit;
  if (!c) return 'ohne Limit';
  const part = (x, one, many) => (x && Number(x.n) ? `${x.n} ${Number(x.n) === 1 ? one : many} in ${x.days} Tagen` : `${many} ohne Limit`);
  return `${part(c.movie, 'Film', 'Filme')}, ${part(c.tv, 'Staffel', 'Staffeln')}`;
}
function vaDraft(id) {
  const d = ui.draft.invite;
  if (!d || d.forInv !== id) ui.draft.invite = { forInv: id, email: '', name: '', pw1: '', pw2: '' };
  return ui.draft.invite;
}
function vaShell(inner) {
  return `<div class="va-wrap"><main class="va-card" id="content"><a class="va-brand" href="#/mockup" data-fid="va:brand">${LOGO}<span>Slouchcat</span></a>${inner}</main></div>`;
}
route('/invite/:id', (pm) => {
  const inv = S.invitations.find((i) => i.id === pm.id);
  if (!inv || inv.state !== 'open') {
    return { type: 'bare', title: 'Einladung nicht gültig', html: vaShell(`<h1 class="va-h1">Diese Einladung ist nicht mehr gültig</h1><p class="va-sub">Der Link wurde schon benutzt, ist abgelaufen oder widerrufen. Bitte die Person, die dich eingeladen hat, um einen neuen Link.</p><div class="va-acts"><a class="btn secondary" href="#/mockup" data-fid="va:back">${icon('chevL', 16)}Alle Mockup-Seiten</a></div>`) };
  }
  const d = vaDraft(inv.id), e = ui.vaErr || {};
  const ss = seerrState();
  const f = (k, label, type, auto, hint) => `<div class="fld"><label for="va:${k}">${label}</label><input id="va:${k}" class="inp" type="${type}" data-bind="ui.draft.invite.${k}" data-fid="va:${k}" value="${esc(d[k])}" autocomplete="${auto}" ${e[k] ? 'aria-invalid="true" aria-describedby="va:' + k + ':e"' : ''} ${k === 'email' ? 'data-autofocus' : ''}>${hint ? `<span class="hint">${hint}</span>` : ''}${e[k] ? `<span class="err" id="va:${k}:e">${e[k]}</span>` : ''}</div>`;
  const facts = `<ul class="va-facts" aria-label="Was diese Einladung erlaubt">${inv.note ? `<li>${icon('note', 16)}<span>«${esc(inv.note)}»</span></li>` : ''}<li>${icon('lock', 16)}<span>Altersstufe ${ageLabel(inv.maxRating)}</span></li><li>${icon(inv.canRequest ? 'wish' : 'ban', 16)}<span>${inv.canRequest ? `Darf Titel anfordern · Limit: ${vaLimitText(inv)}` : 'Darf keine Titel anfordern'}</span></li><li>${icon('clock', 16)}<span>Gültig bis ${fmtDate(inv.until)}</span></li></ul>`;
  const seerr = vaSeerrOn() ? `<div class="va-seerr"><b>Dein Login gilt auch für Seerr <span class="newtag">NEU</span></b><p>${ss.k === 'ok' ? 'Slouchcat eröffnet dir dort ein Konto mit derselben E-Mail-Adresse und demselben Passwort. Was du in Slouchcat anforderst, erscheint dort unter deinem Namen.' : 'Mit derselben E-Mail-Adresse und demselben Passwort. Seerr ist gerade nicht erreichbar: Dein Konto dort folgt beim nächsten Abgleich, das Passwort bei deiner nächsten Anmeldung.'}</p></div>` : '';
  const html = vaShell(`<h1 class="va-h1">Einladung annehmen</h1><p class="va-sub">Lege dein Konto an. Der Name wird dein erstes Profil; weitere Profile legst du später selbst an.</p>${facts}
    <div class="va-form">${f('email', 'E-Mail', 'email', 'email')}${f('name', 'Name des Profils', 'text', 'nickname')}${f('pw1', 'Passwort', 'password', 'new-password', 'Mindestens 10 Zeichen.')}${f('pw2', 'Passwort wiederholen', 'password', 'new-password')}</div>
    ${seerr}<div class="va-acts"><button type="button" class="btn primary lg" data-act="vaAccept" data-id="${inv.id}" data-fid="va:ok">Konto anlegen</button></div>`);
  return { type: 'bare', title: 'Einladung annehmen', html };
});
ACT.vaAccept = (dd) => {
  const d = vaDraft(dd.id), e = {};
  const email = d.email.trim(), name = d.name.trim();
  if (!/^[^@\s]+@[^@\s]+$/.test(email)) e.email = 'Gib eine gültige E-Mail-Adresse ein.';
  else if (S.accounts.some((a) => a.email.toLowerCase() === email.toLowerCase())) e.email = 'Mit dieser E-Mail-Adresse gibt es schon ein Konto.';
  if (!name) e.name = 'Gib einen Namen für dein Profil ein.';
  if (d.pw1.length < 10) e.pw1 = 'Mindestens 10 Zeichen.';
  else if (d.pw1 !== d.pw2) e.pw2 = 'Die beiden Passwörter sind nicht gleich.';
  ui.vaErr = e;
  if (Object.keys(e).length) { render(); return; }
  const res = acceptInvitation(dd.id, { email, name });
  delete ui.draft.invite; ui.vaErr = {};
  if (!res) { toast('Diese Einladung ist nicht mehr gültig.', 'bad'); render(); return; }
  const st = accSeerr(account(res.accountId));
  switchProfile(res.profileId);
  go('#/');
  /* the same sentence whether the Seerr user is new or was linked: the page never says whether Seerr knew the address */
  const msg = st.state === 'created' || st.state === 'linked' ? 'In Seerr hast du jetzt ein Konto mit derselben E-Mail-Adresse.'
    : st.state === 'waiting' ? 'Dein Konto in Seerr ist noch nicht bereit. Bis dahin kannst du noch keine Titel anfordern.' : '';
  toast(`Willkommen, ${esc(name)}. Dein Konto ist angelegt.${msg ? ' ' + msg : ''}`, st.state === 'waiting' ? 'warn' : 'ok', 7000);
};
