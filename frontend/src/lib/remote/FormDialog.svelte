<script lang="ts">
	// Insertion › Formulaire, sur le tableau actif (conception :
	// docs/conception/partager-comme-formulaire.md, lot 1). Quand une
	// condition manque (§2.2), la boîte le dit et propose l'action à faire
	// d'abord (publier, ajouter au classeur partagé, réessayer). Sinon, le
	// concepteur : à gauche les onglets Champs, Apparence et Accès, à droite
	// l'aperçu tiré de la même définition que la page du serveur. En bas, les
	// vérifications (en orange), Enregistrer le brouillon et Ouvrir le
	// formulaire ; une fois ouvert, son lien et son QR code.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import HelpLink from '#lib/help/HelpLink.svelte';
	import { openOutside } from '#lib/ai/links.js';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import FormPreview from './FormPreview.svelte';
	import FormQR from './FormQR.svelte';
	import FormPrefill from './FormPrefill.svelte';
	import {
		PALETTE,
		STATE_LABELS,
		accentColor,
		buildDefinition,
		checks,
		columnNames,
		copiedLabel,
		draftFromForm,
		fieldColumns,
		isoDay,
		listFor,
		newDraft,
		paramName,
		privacyTemplate,
		reasonAction,
		widgetsFor,
		type Access,
		type Draft,
		type DraftField,
		type FormColumn,
		type FormInfo,
		type ServerForm
	} from './forms';
	import './shareForm.css';

	interface Props {
		table: string;
		ondone: () => void;
		/** Publier le classeur (la boîte se ferme ; elle revient une fois publié). */
		onpublish?: () => void;
		/** Ajouter le tableau au classeur partagé (la boîte revient ensuite). */
		onaddshared?: () => void;
		/** Message de la barre d'état. */
		onnotice?: (msg: string) => void;
	}
	let { table, ondone, onpublish, onaddshared, onnotice }: Props = $props();

	const HELP = 'fichiers#partager-un-tableau-comme-formulaire';
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	let info = $state<FormInfo | null>(null);
	let error = $state('');
	let busy = $state(false);
	let step = $state<'reason' | 'list' | 'design' | 'done'>('design');
	let draft = $state<Draft | null>(null);
	/** Formulaire du serveur en cours de conception (null : pas encore créé). */
	let form = $state<{ id: string; code: string; url: string; version: number; state: string } | null>(null);
	let tab = $state<'fields' | 'appearance' | 'access'>('fields');
	let device = $state<'phone' | 'desktop'>('phone');
	let open = $state<string | null>(null);
	let prefill = $state(false);
	/** Audience : personnes et équipes du serveur (actuelles, à ajouter, à retirer). */
	let audience = $state<{ principal: string; kind: string; name: string }[]>([]);
	let adds = $state<{ email: string; team: string; name: string }[]>([]);
	let removed = $state<string[]>([]);
	let email = $state('');
	let team = $state('');
	/** Images choisies sur ce poste (aperçu), ou lues sur le serveur. */
	let logoSrc = $state('');
	let bannerSrc = $state('');
	let dragging = $state<string | null>(null);

	const cols = $derived(info?.columns ?? []);
	const def = $derived(draft ? buildDefinition(draft) : null);
	const verdict = $derived(draft && info ? checks(draft, info) : { errors: [], warnings: [] });
	const names = $derived(columnNames(cols));
	const colOf = (id?: string): FormColumn | undefined => cols.find((c) => c.columnId === id);
	const manageable = $derived((info?.forms ?? []).filter((f) => f.manageable));
	const hasReceived = $derived(cols.some((c) => /^reçu le$/i.test(c.name.trim())));

	onMount(load);

	async function load() {
		error = '';
		try {
			const i = (await DocumentService.FormInfo(table)) as unknown as FormInfo;
			info = i;
			if (i.reason) step = 'reason';
			else if (manageable.length && !draft) step = 'list';
			else if (!draft) draft = newDraft(i);
		} catch (e) {
			error = err(e);
		}
	}

	/** Action proposée par le message du §2.2. */
	function act(a: 'publish' | 'addshared' | 'retry') {
		if (a === 'retry') {
			info = null;
			void load();
			return;
		}
		ondone();
		if (a === 'publish') onpublish?.();
		else onaddshared?.();
	}

	function edit(f: ServerForm) {
		if (!info) return;
		draft = draftFromForm(info, f);
		form = { id: f.id, code: f.code, url: f.url, version: f.version ?? 1, state: f.state };
		audience = f.audience ?? [];
		const base = f.url.replace(/\/f\/[^/]*$/, '');
		logoSrc = draft.appearance.logo ? `${base}/f/${f.code}/img/${draft.appearance.logo}` : '';
		bannerSrc = draft.appearance.banner ? `${base}/f/${f.code}/img/${draft.appearance.banner}` : '';
		step = 'design';
	}

	function fresh() {
		if (!info) return;
		draft = newDraft(info);
		form = null;
		audience = [];
		step = 'design';
	}

	async function copyLink(u: string) {
		try {
			await navigator.clipboard.writeText(u);
			onnotice?.('Lien du formulaire copié');
		} catch (e) {
			error = err(e);
		}
	}

	/** Enregistre le brouillon (création ou nouvelle version) ; faux si refusé. */
	async function save(): Promise<boolean> {
		if (!info || !draft || !def) return false;
		error = '';
		busy = true;
		try {
			const res = await DocumentService.FormSave({
				server: info.server,
				workbookId: info.workbookId,
				tableId: info.tableId,
				id: form?.id ?? '',
				version: form?.version ?? 0,
				spec: { definition: def, access: draft.access, audience: [], max_per_respondent: draft.maxPerRespondent ?? undefined } as never,
				add: adds.map(({ email, team }) => ({ email, team, predicate: '' })),
				remove: removed,
				names
			});
			form = { id: res.id, code: res.code || form?.code || '', url: res.url || form?.url || '', version: res.version, state: form?.state ?? 'draft' };
			const refused = res.refused ?? [];
			audience = [...audience.filter((a) => !removed.includes(a.principal)), ...adds.filter((a) => !refused.some((r) => r.startsWith(a.email || a.team))).map((a) => ({ principal: a.team || a.email, kind: a.team ? 'team' : 'user', name: a.name || a.email }))];
			adds = [];
			removed = [];
			if (refused.length) error = `Formulaire enregistré, mais une partie de l’audience est refusée : ${refused.join(' ; ')}`;
			return true;
		} catch (e) {
			error = err(e);
			return false;
		} finally {
			busy = false;
		}
	}

	async function saveDraft() {
		if (await save()) onnotice?.(`Brouillon du formulaire « ${draft?.appearance.title} » enregistré`);
	}

	async function openForm() {
		if (!info || verdict.errors.length) return;
		if (!(await save()) || !form) return;
		busy = true;
		try {
			await DocumentService.FormSetState(info.server, form.id, 'open', names);
			form.state = 'open';
			step = 'done';
			onnotice?.(`Formulaire « ${draft?.appearance.title} » ouvert`);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	async function browserPreview() {
		if (!info || !(await save()) || !form) return;
		try {
			openOutside(await DocumentService.FormPreviewURL(info.server, form.id));
		} catch (e) {
			error = err(e);
		}
	}

	// --- Champs ---------------------------------------------------------

	function addSection() {
		if (!draft) return;
		draft.fields = [...draft.fields, { uid: `s${Date.now()}`, include: true, kind: 'section', title: 'Nouvelle section' }];
	}

	function drop(target: string) {
		if (!draft || !dragging || dragging === target) return;
		const from = draft.fields.findIndex((f) => f.uid === dragging);
		const to = draft.fields.findIndex((f) => f.uid === target);
		const fs = [...draft.fields];
		const [f] = fs.splice(from, 1);
		fs.splice(to, 0, f);
		draft.fields = fs;
		dragging = null;
	}

	function move(f: DraftField, by: number) {
		if (!draft) return;
		const i = draft.fields.indexOf(f);
		const j = i + by;
		if (j < 0 || j >= draft.fields.length) return;
		const fs = [...draft.fields];
		[fs[i], fs[j]] = [fs[j], fs[i]];
		draft.fields = fs;
	}

	/** Mode de la valeur par défaut d'un champ. */
	function defaultMode(f: DraftField): string {
		if (!f.default) return '';
		return f.default.token ?? 'fixed';
	}
	function setDefault(f: DraftField, mode: string) {
		f.default = mode === '' ? null : mode === 'fixed' ? { fixed: '' } : { token: mode };
		if (!f.default) f.hidden = false;
	}

	/** « Mettre à jour » une liste copiée : relue dans le classeur. */
	async function refreshList(f: DraftField) {
		await load();
		const c = colOf(f.column);
		if (c) f.list = listFor(c, isoDay(new Date()));
	}

	function numberOrNull(v: string): number | null {
		const n = Number(v.replace(',', '.'));
		return v.trim() === '' || Number.isNaN(n) ? null : n;
	}

	// --- Valeurs posées par le serveur ------------------------------------

	const SERVER_TOKENS = [
		{ value: 'received_at', label: 'Reçu le (date et heure)' },
		{ value: 'respondent', label: 'Répondant (adresse)' },
		{ value: 'form_title', label: 'Formulaire (son titre)' },
		{ value: 'next_number', label: 'Numéro suivant' },
		{ value: 'fixed', label: 'Valeur fixe' }
	];
	/** Colonnes libres pour une valeur posée par le serveur (ni champ coché, ni déjà posée). */
	const freeColumns = $derived(
		draft ? cols.filter((c) => !draft!.fields.some((f) => f.include && fieldColumns(f).includes(c.columnId)) && !draft!.serverValues.some((v) => v.column === c.columnId) && c.columnId !== draft!.choice?.column) : []
	);

	function addServerValue(column = freeColumns[0]?.columnId ?? '', token = 'received_at') {
		if (!draft || !column) return;
		draft.serverValues = [...draft.serverValues, { column, value: token === 'fixed' ? { fixed: '' } : { token } }];
	}

	async function addReceivedColumn() {
		if (!info) return;
		busy = true;
		error = '';
		try {
			info = (await DocumentService.FormAddColumn(table, 'Reçu le', 'date')) as unknown as FormInfo;
			const c = (info.columns ?? []).find((x) => /^reçu le$/i.test(x.name.trim()));
			if (c) addServerValue(c.columnId, 'received_at');
			onnotice?.('Colonne « Reçu le » ajoutée au tableau');
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	// --- Images -----------------------------------------------------------

	async function pickImage(kind: 'logo' | 'banner', e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || !info || !draft) return;
		const limit = kind === 'logo' ? 200 << 10 : 1 << 20;
		if (file.size > limit) {
			error = `Image trop lourde : ${kind === 'logo' ? '200 Ko pour un logo' : '1 Mo pour une bannière'} au plus.`;
			return;
		}
		if (!form && !(await save())) return;
		busy = true;
		try {
			const b64 = await new Promise<string>((ok, ko) => {
				const r = new FileReader();
				r.onload = () => ok(String(r.result).replace(/^data:[^,]*,/, ''));
				r.onerror = () => ko(new Error('image illisible'));
				r.readAsDataURL(file);
			});
			const id = await DocumentService.FormImage(info.server, form!.id, kind, file.name, b64);
			draft.appearance[kind] = id;
			const src = URL.createObjectURL(file);
			if (kind === 'logo') logoSrc = src;
			else bannerSrc = src;
			busy = false;
			await save();
		} catch (e2) {
			error = err(e2);
		} finally {
			busy = false;
		}
	}

	function dropImage(kind: 'logo' | 'banner') {
		if (!draft) return;
		draft.appearance[kind] = '';
		if (kind === 'logo') logoSrc = '';
		else bannerSrc = '';
	}

	// --- Accès --------------------------------------------------------------

	function addEmail() {
		const e = email.trim();
		if (!e || adds.some((a) => a.email === e)) return;
		adds = [...adds, { email: e, team: '', name: e }];
		email = '';
	}
	function addTeam() {
		const t = info?.teams?.find((x) => x.id === team);
		if (!t || adds.some((a) => a.team === t.id) || audience.some((a) => a.principal === t.id)) return;
		adds = [...adds, { email: '', team: t.id, name: t.name }];
		team = '';
	}

	const ACCESS: { value: Access; label: string; detail: string }[] = $derived([
		{ value: 'members', label: 'Membres du classeur', detail: 'Toute personne qui a un rôle sur le classeur, lecteurs compris. Connexion Lattis.' },
		{ value: 'audience', label: 'Personnes et équipes choisies', detail: 'Elles répondent sans devenir membres du classeur. Connexion Lattis.' },
		{ value: 'server_accounts', label: `Toute personne disposant d’un compte sur ${info?.serverName || 'ce serveur'}`, detail: 'Utile pour un formulaire interne à toute l’organisation. Connexion Lattis.' },
		{ value: 'public', label: 'Lien public', detail: info?.public ? 'Toute personne qui a le lien, sans connexion : bientôt dans Lattis.' : 'Désactivé sur ce serveur (réglage de son propriétaire).' }
	]);
</script>

<div class="back" role="presentation" use:backdrop={() => !busy && ondone()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		class:wide={step === 'design'}
		role="dialog"
		aria-modal="true"
		aria-labelledby="fd-title"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy && !e.defaultPrevented && !prefill) ondone();
		}}
	>
		<header>
			<Icon name="text-input" size={18} /><span id="fd-title" class="title">{form ? `Formulaire « ${draft?.appearance.title} »` : `Partager « ${table} » comme formulaire`}</span>
			{#if form}<span class="state s-{form.state}">{STATE_LABELS[form.state] ?? form.state}</span>{/if}
			<HelpLink target={HELP} label="Aide : partager comme formulaire" />
			<button type="button" class="close" aria-label="Fermer" title="Fermer (Échap)" disabled={busy} onclick={ondone}>✕</button>
		</header>

		{#if !info}
			<div class="sf-body"><p class={error ? 'sf-error' : 'sf-hint'}>{error || 'Lecture du tableau…'}</p></div>
			<footer class="sf-foot"><span class="sf-grow"></span><button type="button" class="sf-btn" onclick={ondone}>Fermer</button></footer>
		{:else if step === 'reason'}
			{@const a = reasonAction(info.reason)}
			<div class="sf-body">
				<p class="lead">{info.message}</p>
				{#if info.reason === 'rights' && info.forms?.length}
					<p class="sf-hint">Formulaires de ce tableau qui vous sont ouverts :</p>
					<ul class="forms">
						{#each info.forms as f (f.id)}
							<li><strong>{f.title}</strong><span class="sf-grow"></span><button type="button" class="sf-btn" onclick={() => copyLink(f.url)}>Copier le lien</button></li>
						{/each}
					</ul>
				{/if}
				{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
			</div>
			<footer class="sf-foot">
				<span class="sf-grow"></span>
				<button type="button" class="sf-btn" onclick={ondone}>{a ? 'Annuler' : 'Fermer'}</button>
				{#if a && (a.action !== 'publish' || onpublish) && (a.action !== 'addshared' || onaddshared)}
					<button type="button" class="sf-btn primary" onclick={() => act(a.action)}>{a.label}</button>
				{/if}
			</footer>
		{:else if step === 'list'}
			<div class="sf-body">
				<p class="lead">Ce tableau a déjà {manageable.length > 1 ? `${manageable.length} formulaires` : 'un formulaire'}. Plusieurs formulaires peuvent remplir le même tableau.</p>
				<ul class="forms">
					{#each manageable as f (f.id)}
						<li>
							<strong>{f.title}</strong>
							<span class="state s-{f.state}">{STATE_LABELS[f.state] ?? f.state}</span>
							<span class="sf-hint">{f.responses} {f.responses > 1 ? 'réponses' : 'réponse'}</span>
							<span class="sf-grow"></span>
							<button type="button" class="sf-btn" onclick={() => copyLink(f.url)}>Copier le lien</button>
							<button type="button" class="sf-btn" onclick={() => edit(f)}>Modifier…</button>
						</li>
					{/each}
				</ul>
				{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
			</div>
			<footer class="sf-foot">
				<span class="sf-grow"></span>
				<button type="button" class="sf-btn" onclick={ondone}>Fermer</button>
				<button type="button" class="sf-btn primary" onclick={fresh}>Nouveau formulaire…</button>
			</footer>
		{:else if step === 'done' && form && draft}
			<div class="sf-body">
				<p class="lead">Le formulaire « {draft.appearance.title} » est ouvert. Les réponses arrivent comme de nouvelles lignes du tableau, en direct.</p>
				<FormQR url={form.url} title={draft.appearance.title} {onnotice} />
				{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
			</div>
			<footer class="sf-foot">
				<button type="button" class="sf-btn" onclick={() => (prefill = true)}>Lien pré-rempli…</button>
				<button type="button" class="sf-btn" onclick={() => (step = 'design')}>Modifier…</button>
				<span class="sf-grow"></span>
				<button type="button" class="sf-btn primary" onclick={ondone}>Fermer</button>
			</footer>
		{:else if draft && def}
			<div class="split">
				<div class="left">
					<div class="tabs" role="tablist">
						<button type="button" role="tab" aria-selected={tab === 'fields'} class:on={tab === 'fields'} onclick={() => (tab = 'fields')}>Champs</button>
						<button type="button" role="tab" aria-selected={tab === 'appearance'} class:on={tab === 'appearance'} onclick={() => (tab = 'appearance')}>Apparence</button>
						<button type="button" role="tab" aria-selected={tab === 'access'} class:on={tab === 'access'} onclick={() => (tab = 'access')}>Accès</button>
					</div>
					<div class="pane">
						{#if tab === 'fields'}
							<ul class="fields">
								{#each draft.fields as f (f.uid)}
									{@const c = colOf(f.column)}
									{@const isChoice = !!draft.choice && f.column === draft.choice.column && f.kind === undefined}
									{#if !isChoice || draft.choice?.mode === 'field'}
										<li
											class="frow"
											class:off={!f.include}
											class:section={f.kind === 'section'}
											class:drag={dragging === f.uid}
											ondragover={(e) => dragging && e.preventDefault()}
											ondrop={(e) => {
												e.preventDefault();
												drop(f.uid);
											}}
										>
											<div class="fline">
												<span
													class="handle"
													draggable="true"
													role="button"
													tabindex="0"
													aria-label="Déplacer (Alt+↑ / Alt+↓)"
													title="Glisser pour déplacer (Alt+↑ / Alt+↓)"
													ondragstart={() => (dragging = f.uid)}
													ondragend={() => (dragging = null)}
													onkeydown={(e) => {
														if (e.altKey && e.key === 'ArrowUp') (e.preventDefault(), move(f, -1));
														if (e.altKey && e.key === 'ArrowDown') (e.preventDefault(), move(f, 1));
													}}>⋮⋮</span
												>
												{#if f.kind === 'section'}
													<input class="sf-input grow" bind:value={f.title} placeholder="Intertitre" aria-label="Intertitre" maxlength="200" />
													<button type="button" class="mini" title="Retirer l’intertitre" aria-label="Retirer l’intertitre" onclick={() => draft && (draft.fields = draft.fields.filter((x) => x !== f))}>✕</button>
												{:else}
													<input type="checkbox" bind:checked={f.include} aria-label="Dans le formulaire" title="Dans le formulaire" />
													<span class="cname" title={f.kind === 'position' ? 'Position du répondant' : `Colonne « ${c?.name} »`}>{f.kind === 'position' ? 'Position' : (c?.name ?? '?')}</span>
													<input class="sf-input grow" bind:value={f.label} aria-label="Libellé" placeholder="Libellé" maxlength="200" disabled={!f.include} />
													{#if f.kind === undefined && c}
														<select class="sf-input wsel" bind:value={f.widget} aria-label="Type de champ" disabled={!f.include}>
															{#each widgetsFor(c) as w (w.value)}<option value={w.value}>{w.label}</option>{/each}
														</select>
													{/if}
													<label class="req" title="Obligatoire"><input type="checkbox" bind:checked={f.required} disabled={!f.include} />Obligatoire</label>
													<button type="button" class="mini" aria-expanded={open === f.uid} title="Réglages du champ" disabled={!f.include} onclick={() => (open = open === f.uid ? null : f.uid)}>{open === f.uid ? '▾' : '▸'}</button>
												{/if}
											</div>
											{#if f.kind === 'section'}
												<textarea class="sf-input sub" rows="1" bind:value={f.description} placeholder="Courte description (facultatif)" aria-label="Description de la section"></textarea>
											{:else if open === f.uid && f.include}
												<div class="more">
													<label>Aide <input class="sf-input" bind:value={f.help} maxlength="500" /></label>
													{#if f.kind === 'position'}
														<p class="sf-hint">Rempli par le navigateur du répondant quand il touche « Ma position » (jamais automatiquement), avec la précision affichée. {f.column ? `Colonne « ${colOf(f.column)?.name} » : « lat, lon ».` : `Colonnes « ${colOf(f.latitude)?.name} » et « ${colOf(f.longitude)?.name} ».`}</p>
													{:else}
														<label>Texte indicatif <input class="sf-input" bind:value={f.placeholder} /></label>
														<div class="pair">
															<label
																>Valeur par défaut
																<select class="sf-input" value={defaultMode(f)} onchange={(e) => setDefault(f, e.currentTarget.value)}>
																	<option value="">Aucune</option>
																	<option value="fixed">Valeur fixe</option>
																	{#if c?.type === 'date' || c?.type === 'datetime'}<option value="today">Aujourd’hui</option>{/if}
																	{#if c?.type === 'datetime'}<option value="now">Maintenant</option>{/if}
																	{#if c?.type === 'integer' || c?.type === 'text'}<option value="year">Année en cours</option>{/if}
																	{#if c?.type === 'text'}<option value="respondent">Adresse du répondant</option>{/if}
																</select>
															</label>
															{#if f.default && f.default.token === undefined}
																<label>Valeur <input class="sf-input" value={String(f.default.fixed ?? '')} oninput={(e) => (f.default = { fixed: e.currentTarget.value })} /></label>
															{/if}
														</div>
														{#if f.default}
															<label class="sf-check"><input type="checkbox" bind:checked={f.hidden} />Caché : la valeur par défaut est posée sans être montrée</label>
														{/if}
														{#if f.widget === 'number'}
															<div class="pair">
																<label>Minimum <input class="sf-input" value={f.min ?? ''} oninput={(e) => (f.min = numberOrNull(e.currentTarget.value))} /></label>
																<label>Maximum <input class="sf-input" value={f.max ?? ''} oninput={(e) => (f.max = numberOrNull(e.currentTarget.value))} /></label>
																<label>Pas <input class="sf-input" value={f.step ?? ''} oninput={(e) => (f.step = numberOrNull(e.currentTarget.value))} /></label>
																<label>Unité <input class="sf-input" bind:value={f.unit} maxlength="10" /></label>
															</div>
														{:else if f.widget === 'text' || f.widget === 'textarea'}
															<div class="pair">
																<label>Longueur maximale <input class="sf-input" type="number" min="1" max="10000" value={f.max_length ?? ''} oninput={(e) => (f.max_length = numberOrNull(e.currentTarget.value))} /></label>
																<label
																	>Format
																	<select class="sf-input" bind:value={f.format}>
																		<option value="">Libre</option>
																		<option value="email">Adresse e-mail</option>
																		<option value="tel">Téléphone</option>
																		<option value="url">Adresse web</option>
																	</select>
																</label>
															</div>
														{/if}
														{#if f.list}
															{#if f.list.table_id}
																<p class="sf-hint">Liste tirée de « {c?.list?.source} », relue par le serveur à chaque affichage ({c?.list?.values?.length ?? 0} valeurs aujourd’hui).</p>
															{:else}
																<label
																	>Valeurs de la liste (une par ligne)
																	<textarea class="sf-input" rows="4" value={(f.list.values ?? []).join('\n')} oninput={(e) => f.list && (f.list = { ...f.list, values: e.currentTarget.value.split('\n') })}></textarea>
																</label>
																{#if f.list.copied_at}
																	<div class="sf-static">
																		<span class="sf-hint">{copiedLabel(f.list.copied_at)} depuis « {f.list.source} »</span>
																		<button type="button" class="sf-btn" onclick={() => refreshList(f)} disabled={busy}>Mettre à jour</button>
																	</div>
																{/if}
															{/if}
														{/if}
														<div class="pair">
															<label>Nom court dans le lien <input class="sf-input" bind:value={f.param} spellcheck="false" placeholder={paramName(f.label ?? '')} /></label>
															<label class="sf-check lock"><input type="checkbox" bind:checked={f.locked} />Verrouillé par le lien</label>
														</div>
													{/if}
												</div>
											{/if}
										</li>
									{/if}
								{/each}
							</ul>
							<button type="button" class="sf-btn" onclick={addSection}>+ Intertitre</button>

							{#if draft.choice}
								{@const ch = draft.choice}
								<h3>Onglet à choix · « {colOf(ch.column)?.name} »</h3>
								<div class="sf-seg" role="radiogroup" aria-label="Valeur de la catégorie">
									<label class:on={ch.mode === 'fixed'}><input type="radio" bind:group={ch.mode} value="fixed" />Valeur fixe</label>
									{#if /^\d{4}$/.test(ch.value)}<label class:on={ch.mode === 'year'}><input type="radio" bind:group={ch.mode} value="year" />Année en cours</label>{/if}
									<label class:on={ch.mode === 'field'}><input type="radio" bind:group={ch.mode} value="field" />Un champ</label>
								</div>
								{#if ch.mode === 'fixed'}<input class="sf-input inline" bind:value={ch.value} aria-label="Valeur fixe de la catégorie" />{/if}
							{/if}

							<h3>Valeurs posées par le serveur</h3>
							<p class="sf-hint">Sans champ visible, chacune sur une colonne choisie. {cols.some((c) => c.auto) ? 'La clé numérotée reçoit le numéro suivant, posé par le serveur.' : ''}</p>
							{#each draft.serverValues as v, i (i)}
								<div class="sv">
									<select class="sf-input" bind:value={v.column} aria-label="Colonne">
										{#each cols.filter((c) => c.columnId === v.column || freeColumns.includes(c)) as c (c.columnId)}<option value={c.columnId}>{c.name}</option>{/each}
									</select>
									<select class="sf-input" value={v.value.token ?? 'fixed'} aria-label="Valeur" onchange={(e) => (v.value = e.currentTarget.value === 'fixed' ? { fixed: '' } : { token: e.currentTarget.value })}>
										{#each SERVER_TOKENS as t (t.value)}<option value={t.value}>{t.label}</option>{/each}
									</select>
									{#if v.value.token === undefined}<input class="sf-input" value={String(v.value.fixed ?? '')} aria-label="Valeur fixe" oninput={(e) => (v.value = { fixed: e.currentTarget.value })} />{/if}
									<button type="button" class="mini" aria-label="Retirer" onclick={() => draft && (draft.serverValues = draft.serverValues.filter((_, j) => j !== i))}>✕</button>
								</div>
							{/each}
							<div class="sf-static">
								<button type="button" class="sf-btn" disabled={!freeColumns.length} onclick={() => addServerValue()}>+ Valeur posée par le serveur</button>
								{#if !hasReceived}<button type="button" class="sf-btn" disabled={busy} onclick={addReceivedColumn} title="Ajoute une colonne au tableau, pour tous les membres">Ajouter une colonne Reçu le</button>{/if}
							</div>
						{:else if tab === 'appearance'}
							<div class="sf-fields">
								<label class="sf-label" for="fd-t">Titre</label>
								<input id="fd-t" class="sf-input" bind:value={draft.appearance.title} maxlength="120" />
								<label class="sf-label" for="fd-i">Introduction</label>
								<div class="sf-field">
									<textarea id="fd-i" class="sf-input" rows="3" bind:value={draft.appearance.intro} maxlength="2000"></textarea>
									<p class="sf-hint">**gras**, *italique*, [lien](https://…), listes avec « - ».</p>
								</div>
								<span class="sf-label">Logo</span>
								<div class="sf-static">
									{#if logoSrc}<img class="thumb" src={logoSrc} alt="Logo" />{:else if !draft.appearance.logo}<span class="sf-hint">Celui du serveur</span>{/if}
									<label class="sf-btn file">Choisir…<input type="file" accept=".png,.jpg,.jpeg,.svg" onchange={(e) => pickImage('logo', e)} disabled={busy} /></label>
									{#if draft.appearance.logo}<button type="button" class="sf-btn" onclick={() => dropImage('logo')}>Retirer</button>{/if}
									<span class="sf-hint">PNG, JPEG ou SVG, 200 Ko au plus</span>
								</div>
								<span class="sf-label">Bannière</span>
								<div class="sf-static">
									{#if bannerSrc}<img class="thumb wide" src={bannerSrc} alt="Bannière" />{/if}
									<label class="sf-btn file">Choisir…<input type="file" accept=".png,.jpg,.jpeg" onchange={(e) => pickImage('banner', e)} disabled={busy} /></label>
									{#if draft.appearance.banner}<button type="button" class="sf-btn" onclick={() => dropImage('banner')}>Retirer</button>{/if}
									<span class="sf-hint">1 Mo au plus ; le serveur retire la position GPS de la photo</span>
								</div>
								<span class="sf-label">Couleur</span>
								<div class="palette" role="radiogroup" aria-label="Couleur d’accent">
									{#each PALETTE as p (p.name)}
										<label class="swatch" class:on={draft.appearance.accent === p.name} title={p.label} style:background={accentColor(p.name)}>
											<input type="radio" bind:group={draft.appearance.accent} value={p.name} aria-label={p.label} />
										</label>
									{/each}
								</div>
								<span class="sf-label">Pages</span>
								<label class="sf-check"><input type="checkbox" bind:checked={draft.appearance.paged} />Une section par page (Suivant, Précédent, barre d’avancement)</label>
								<label class="sf-label" for="fd-b">Bouton</label>
								<input id="fd-b" class="sf-input" bind:value={draft.appearance.submit_label} maxlength="40" />
								<label class="sf-label" for="fd-m">Remerciement</label>
								<div class="sf-field">
									<textarea id="fd-m" class="sf-input" rows="2" bind:value={draft.appearance.thanks}></textarea>
									<label class="sf-check"><input type="checkbox" bind:checked={draft.appearance.another} />Bouton « Envoyer une autre réponse »</label>
									<p class="sf-hint">Gras et italique seulement : ni lien ni redirection.</p>
								</div>
								<label class="sf-label" for="fd-c">Fermé</label>
								<input id="fd-c" class="sf-input" bind:value={draft.appearance.closed_message} maxlength="500" placeholder="Message quand le formulaire est fermé, plein ou pas encore ouvert" />
								<label class="sf-label" for="fd-p">Données</label>
								<div class="sf-field">
									<textarea id="fd-p" class="sf-input" rows="3" bind:value={draft.appearance.privacy} maxlength="1000" placeholder="Mention de protection des données, au-dessus du bouton"></textarea>
									{#if !draft.appearance.privacy}<button type="button" class="sf-more" onclick={() => draft && (draft.appearance.privacy = privacyTemplate(info?.serverName ?? ''))}>Partir du modèle</button>{/if}
								</div>
								<label class="sf-label" for="fd-l">Langue</label>
								<select id="fd-l" class="sf-input" bind:value={draft.lang}>
									<option value="fr">Français</option>
									<option value="en">Anglais</option>
								</select>
							</div>
						{:else}
							<div class="ways" role="radiogroup" aria-label="Qui répond">
								{#each ACCESS as a (a.value)}
									<label class="way" class:on={draft.access === a.value} class:dis={a.value === 'public'}>
										<input type="radio" bind:group={draft.access} value={a.value} disabled={a.value === 'public'} />
										<span><strong>{a.label}</strong><span class="sf-hint">{a.detail}</span></span>
									</label>
								{/each}
							</div>
							{#if draft.access === 'audience'}
								<h3>Personnes et équipes</h3>
								<ul class="aud">
									{#each audience.filter((a) => !removed.includes(a.principal)) as a (a.principal)}
										<li>{a.kind === 'team' ? 'Équipe ' : ''}{a.name}<button type="button" class="mini" aria-label="Retirer {a.name}" onclick={() => (removed = [...removed, a.principal])}>✕</button></li>
									{/each}
									{#each adds as a, i (i)}
										<li>{a.team ? 'Équipe ' : ''}{a.name} <span class="sf-hint">(à ajouter)</span><button type="button" class="mini" aria-label="Retirer {a.name}" onclick={() => (adds = adds.filter((_, j) => j !== i))}>✕</button></li>
									{/each}
								</ul>
								<div class="sf-inline">
									<input class="sf-input" type="email" bind:value={email} placeholder="adresse@exemple.fr" aria-label="Adresse" onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addEmail())} />
									<button type="button" class="sf-btn" onclick={addEmail}>Ajouter</button>
									{#if info.teams?.length}
										<select class="sf-input" bind:value={team} aria-label="Équipe">
											<option value="">Équipe…</option>
											{#each info.teams as t (t.id)}<option value={t.id}>{t.name}</option>{/each}
										</select>
										<button type="button" class="sf-btn" onclick={addTeam} disabled={!team}>Ajouter</button>
									{/if}
								</div>
							{/if}
							<p class="sf-hint note">Les réponses sont écrites sous l’identité du formulaire « Formulaire : {draft.appearance.title} » : le répondant n’a aucun droit sur le classeur.</p>
						{/if}
					</div>
				</div>
				<div class="right">
					<div class="ptools">
						<div class="sf-seg" role="radiogroup" aria-label="Largeur de l’aperçu">
							<label class:on={device === 'phone'}><input type="radio" bind:group={device} value="phone" />Téléphone</label>
							<label class:on={device === 'desktop'}><input type="radio" bind:group={device} value="desktop" />Ordinateur</label>
						</div>
						<span class="sf-grow"></span>
						<button type="button" class="sf-more" onclick={browserPreview} disabled={busy} title="Enregistre le brouillon et ouvre la vraie page du serveur (lien valable 10 minutes, réponses non enregistrées)">Ouvrir l’aperçu dans le navigateur</button>
					</div>
					<div class="pscroll">
						<FormPreview {def} columns={cols} {device} serverName={info.serverName} logo={logoSrc} banner={bannerSrc} />
					</div>
				</div>
			</div>
			<footer class="sf-foot design">
				<div class="checks" aria-live="polite">
					{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
					{#each verdict.errors as m (m)}<p class="sf-error">{m}</p>{/each}
					{#each verdict.warnings as m (m)}<p class="warn">{m}</p>{/each}
				</div>
				<button type="button" class="sf-btn" disabled={!form?.url || busy} onclick={() => (prefill = true)} title={form?.url ? 'Liens qui remplissent des champs, et leurs QR codes' : 'Enregistrez d’abord le brouillon'}>Lien pré-rempli…</button>
				{#if form?.url}<button type="button" class="sf-btn" onclick={() => form && copyLink(form.url)}>Copier le lien</button>{/if}
				<button type="button" class="sf-btn" onclick={saveDraft} disabled={busy}>{busy ? 'Enregistrement…' : 'Enregistrer le brouillon'}</button>
				<button type="button" class="sf-btn primary" onclick={openForm} disabled={busy || verdict.errors.length > 0}>{form?.state === 'open' ? 'Enregistrer' : 'Ouvrir le formulaire'}</button>
			</footer>
		{/if}
	</div>
</div>
{#if prefill && form?.url && def}
	<FormPrefill url={form.url} title={def.appearance.title} fields={def.fields} columns={cols} onclose={() => (prefill = false)} {onnotice} />
{/if}

<style>
	.back {
		position: fixed;
		inset: 0;
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: var(--z-dialog);
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(640px, calc(100vw - 32px));
		max-height: min(88vh, calc(100vh - 32px));
		min-height: 0;
		display: flex;
		flex-direction: column;
	}
	.dialog.wide {
		width: min(1180px, calc(100vw - 32px));
		height: min(88vh, calc(100vh - 32px));
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 10px 10px 18px;
		font-weight: 600;
		border-bottom: 1px solid #e5e7eb;
		flex: none;
		--icon-accent: #217346;
	}
	.title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	header :global(.help-link) {
		margin: 0;
	}
	.close {
		border: none;
		background: none;
		width: 26px;
		height: 26px;
		border-radius: 4px;
		color: #6b7280;
		cursor: pointer;
		font: inherit;
	}
	.close:hover:not(:disabled) {
		background: #f0f1f3;
		color: #1f2328;
	}
	.state {
		padding: 1px 8px;
		border-radius: 10px;
		font-size: 11px;
		font-weight: 600;
		background: #f3f4f6;
		color: #4b5563;
	}
	.state.s-open {
		background: #e7f3ec;
		color: #217346;
	}
	.state.s-paused {
		background: #fef3c7;
		color: #b45309;
	}
	.lead {
		margin: 0 0 12px;
	}
	.forms {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.forms li {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 0;
		border-bottom: 1px solid #eef0f2;
	}
	.sf-grow {
		flex: 1;
	}
	.split {
		flex: 1;
		min-height: 0;
		display: grid;
		grid-template-columns: minmax(0, 56fr) minmax(0, 44fr);
	}
	.left {
		min-height: 0;
		display: flex;
		flex-direction: column;
		border-right: 1px solid #e5e7eb;
	}
	.tabs {
		display: flex;
		gap: 2px;
		padding: 6px 12px 0;
		border-bottom: 1px solid #e5e7eb;
		flex: none;
	}
	.tabs button {
		padding: 6px 14px;
		border: none;
		border-bottom: 2px solid transparent;
		background: none;
		font: inherit;
		color: #4b5563;
		cursor: pointer;
	}
	.tabs button.on {
		color: #217346;
		border-bottom-color: #217346;
		font-weight: 600;
	}
	.pane {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 10px 14px 14px;
	}
	.pane h3 {
		margin: 16px 0 6px;
		font-size: 13px;
	}
	.fields {
		list-style: none;
		margin: 0 0 8px;
		padding: 0;
	}
	.frow {
		padding: 4px 0;
		border-bottom: 1px solid #eef0f2;
	}
	.frow.section {
		background: #f6f8f7;
	}
	.frow.drag {
		opacity: 0.5;
	}
	.fline {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.frow.off .cname {
		color: #9ca3af;
	}
	.handle {
		cursor: grab;
		color: #9ca3af;
		padding: 0 2px;
		user-select: none;
	}
	.cname {
		width: 92px;
		flex: none;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
		color: #374151;
	}
	.grow {
		flex: 1;
		min-width: 0;
	}
	.wsel {
		width: 130px;
		flex: none;
	}
	.req {
		display: flex;
		align-items: center;
		gap: 3px;
		font-size: 12px;
		white-space: nowrap;
	}
	.mini {
		border: none;
		background: none;
		width: 22px;
		height: 22px;
		border-radius: 3px;
		color: #6b7280;
		cursor: pointer;
		flex: none;
	}
	.mini:hover:not(:disabled) {
		background: #f0f1f3;
	}
	.sub {
		margin: 4px 0 0 22px;
		width: calc(100% - 22px);
		resize: vertical;
	}
	.more {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin: 6px 0 6px 22px;
		padding: 8px 10px;
		background: #f9fafb;
		border-radius: 4px;
	}
	.more label {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-size: 12px;
		color: #374151;
	}
	.more label.sf-check {
		flex-direction: row;
	}
	.pair {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.pair > label {
		flex: 1;
		min-width: 110px;
	}
	.lock {
		align-self: flex-end;
		padding-bottom: 5px;
	}
	.sv {
		display: flex;
		gap: 6px;
		align-items: center;
		margin-bottom: 6px;
	}
	.inline {
		margin-top: 6px;
		max-width: 200px;
	}
	.palette {
		display: flex;
		gap: 6px;
		padding-top: 2px;
	}
	.swatch {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		cursor: pointer;
		box-shadow: inset 0 0 0 2px #fff;
		border: 2px solid transparent;
	}
	.swatch.on {
		border-color: #1f2328;
	}
	.swatch:has(input:focus-visible) {
		outline: 2px solid rgba(33, 115, 70, 0.45);
		outline-offset: 2px;
	}
	.swatch input,
	.file input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
	}
	.file {
		position: relative;
		cursor: pointer;
	}
	.thumb {
		height: 28px;
		max-width: 80px;
		object-fit: contain;
		border: 1px solid #e5e7eb;
	}
	.thumb.wide {
		max-width: 140px;
	}
	.ways {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.way {
		display: flex;
		gap: 10px;
		align-items: flex-start;
		padding: 9px 12px;
		border: 1px solid #d6d9dd;
		border-radius: 6px;
		cursor: pointer;
	}
	.way > span {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.way.on {
		border-color: #217346;
		background: #f3f8f5;
	}
	.way.dis {
		cursor: default;
		opacity: 0.65;
	}
	.way input {
		margin-top: 3px;
		accent-color: #217346;
	}
	.aud {
		list-style: none;
		margin: 0 0 8px;
		padding: 0;
	}
	.aud li {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 0;
	}
	.note {
		margin-top: 14px;
	}
	.right {
		min-height: 0;
		display: flex;
		flex-direction: column;
		background: #f3f4f6;
	}
	.ptools {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		flex: none;
	}
	.pscroll {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 4px 12px 16px;
	}
	.sf-foot.design {
		align-items: flex-end;
	}
	.checks {
		flex: 1;
		min-width: 0;
		max-height: 72px;
		overflow: auto;
	}
	.checks p {
		margin: 0;
		font-size: 12px;
	}
	.warn {
		color: #b45309;
	}
	@media (max-width: 860px) {
		.split {
			grid-template-columns: minmax(0, 1fr);
			overflow: auto;
		}
		.left {
			border-right: none;
		}
	}
</style>
