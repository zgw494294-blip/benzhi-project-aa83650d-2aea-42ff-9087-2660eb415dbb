const state = {batches: [], current: null, viewer: {actorId: 'manager-1', role: 'manager'}};
const $ = selector => document.querySelector(selector);
const key = () => crypto.randomUUID();
const headers = () => ({'Content-Type': 'application/json', 'Idempotency-Key': key()});

async function api(path, options = {}) {
  const response = await fetch(path, options);
  const body = await response.json().catch(() => ({error: '响应格式错误'}));
  if (!response.ok) {
    const error = new Error(body.error || `HTTP ${response.status}`);
    error.payload = body;
    throw error;
  }
  return body;
}

function toast(message) {
  const element = $('#toast');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => element.style.display = 'none', 3500);
}

function errorText(error) {
  const rows = error.payload?.errors || [];
  if (!rows.length) return error.message;
  return `${error.message}\n${rows.map(item => `第 ${item.row || '-'} 行 · ${item.field}：${item.message}`).join('\n')}`;
}

async function refresh() {
  try {
    const data = await api('/api/batches');
    state.batches = data.batches;
    $('#connection').textContent = '本地服务已连接';
    $('#newBatch').disabled = state.viewer.role !== 'manager';
    renderList();
    if (state.current) await openBatch(state.current.batch.id);
  } catch (error) {
    $('#connection').textContent = '连接失败';
    toast(errorText(error));
  }
}

function renderList() {
  const list = $('#batchList');
  list.replaceChildren(...state.batches.map(batch => {
    const button = document.createElement('button');
    button.className = 'batch-card';
    button.innerHTML = `<strong>${escapeHTML(batch.title)}</strong><small>${escapeHTML(batch.siteCode)} · ${statusName(batch.status)} · v${batch.version}</small>`;
    button.onclick = () => openBatch(batch.id);
    return button;
  }));
}

async function openBatch(id) {
  try {
    const query = new URLSearchParams(state.viewer);
    state.current = await api(`/api/batches/${id}?${query}`);
    renderWorkspace();
  } catch (error) {
    toast(errorText(error));
  }
}

function renderWorkspace() {
  const {batch, submissions, openQueue, reannotationTasks, reannotationProgress} = state.current;
  const clips = batch.clips || [];
  const quality = batch.lastQuality;
  $('#workspace').innerHTML = `
    <div class="statusbar"><div><p class="eyebrow">${escapeHTML(batch.siteCode)}</p><h2>${escapeHTML(batch.title)}</h2></div><span class="pill">${statusName(batch.status)} · v${batch.version}</span></div>
    <div class="facts">
      <div class="fact"><small>片段</small><strong>${clips.length}</strong></div>
      <div class="fact"><small>允许物种</small><strong>${escapeHTML((batch.allowedSpeciesCodes || []).join(', ') || '未配置')}</strong></div>
      <div class="fact"><small>未决分歧</small><strong>${openQueue.length}</strong></div>
      <div class="fact"><small>待返标 / 进行中</small><strong>${reannotationProgress.pending} / ${reannotationProgress.inProgress}</strong></div>
    </div>
    ${actions(batch)}
    ${taskPanel(reannotationTasks, reannotationProgress)}
    ${clipPanel(batch, clips, submissions, reannotationTasks)}
    ${disputePanel(openQueue)}
    ${qualityPanel(quality)}
    ${manifestPanel(batch)}
  `;
  bindActions(batch, clips, openQueue, submissions, reannotationTasks);
}

function actions(batch) {
  const manager = state.viewer.role === 'manager';
  const reviewer = state.viewer.role === 'reviewer';
  const publisher = state.viewer.role === 'release_manager';
  return `<div class="panel toolbar">
    <button id="scope" ${!manager || batch.status !== 'draft' ? 'disabled' : ''}>配置范围</button>
    <button id="addClip" ${!manager || batch.status !== 'draft' ? 'disabled' : ''}>登记片段</button>
    <button id="bulkClips" ${!manager || batch.status !== 'draft' ? 'disabled' : ''}>批量登记</button>
    <button id="freeze" class="primary" ${!manager || batch.status !== 'draft' ? 'disabled' : ''}>冻结范围</button>
    <button id="check" ${!(reviewer || publisher) || batch.status === 'draft' || batch.status === 'released' ? 'disabled' : ''}>执行质量检查</button>
    <button id="release" class="primary" ${!publisher || batch.status !== 'ready' ? 'disabled' : ''}>封存发布</button>
  </div>`;
}

function taskPanel(tasks, progress) {
  if (!tasks.length && state.viewer.role !== 'reviewer' && state.viewer.role !== 'annotator') return '';
  const rows = tasks.map(task => `<tr>
    <td>${escapeHTML(task.clipId)}<br><small>${escapeHTML(task.originalKind)} · ${escapeHTML(task.originalBasis.explanation || '无匹配说明')}</small></td>
    <td>${escapeHTML(task.targetAnnotator)} / R${task.round}</td>
    <td>${escapeHTML(task.reason)}${task.revisionReason ? `<br><small>修订：${escapeHTML(task.revisionReason)}</small>` : ''}</td>
    <td>${statusName(task.status)}</td>
    <td><button data-task-edit="${attr(task.id)}" ${state.viewer.role !== 'annotator' || task.status === 'closed' ? 'disabled' : ''}>打开返标</button></td>
  </tr>`).join('');
  return `<div class="panel"><h3>定向返标任务</h3><p>待处理 ${progress.pending}，进行中 ${progress.inProgress}，已闭环 ${progress.closed}。</p>${rows ? `<table><thead><tr><th>原分歧与匹配依据</th><th>目标 / 轮次</th><th>退回与修订依据</th><th>状态</th><th>操作</th></tr></thead><tbody>${rows}</tbody></table>` : '<p>当前身份没有返标待办。</p>'}</div>`;
}

function clipPanel(batch, clips, submissions, tasks) {
  const annotator = state.viewer.role === 'annotator';
  return `<div class="panel"><h3>片段标注器</h3><table><thead><tr><th>序号 / 来源</th><th>时长</th><th>可见提交</th><th>操作</th></tr></thead><tbody>${clips.map(clip => {
    const visible = submissions.filter(item => item.clipId === clip.id);
    const mine = visible.filter(item => item.annotatorId === state.viewer.actorId);
    const activeTask = tasks.find(task => task.clipId === clip.id && task.targetAnnotator === state.viewer.actorId && task.status !== 'closed');
    const locked = mine.some(item => item.status === 'submitted') && !activeTask;
    return `<tr><td>${clip.sequence} · ${escapeHTML(clip.sourceName)}<br><small>${escapeHTML(clip.id)}</small></td><td>${clip.durationMs}ms</td><td>${visible.map(item => `${escapeHTML(item.annotatorId)} / R${item.round} / ${statusName(item.status)} / ${item.events.length} 事件`).join('<br>') || '尚无'}</td><td><button data-annotate="${attr(clip.id)}" ${!annotator || batch.status === 'draft' || batch.status === 'released' || locked ? 'disabled' : ''}>${activeTask ? '处理返标' : '编辑草稿'}</button> <button data-remove="${attr(clip.id)}" ${state.viewer.role !== 'manager' || batch.status !== 'draft' ? 'disabled' : ''}>移除</button></td></tr>`;
  }).join('')}</tbody></table></div>`;
}

function disputePanel(queue) {
  const rows = queue.map(dispute => `<tr><td>${escapeHTML(dispute.kind)}</td><td>${escapeHTML(dispute.clipId)}</td><td>${escapeHTML(dispute.basis.explanation)}</td><td><button data-resolve="${attr(dispute.id)}" ${state.viewer.role !== 'reviewer' ? 'disabled' : ''}>仲裁</button></td></tr>`).join('');
  return `<div class="panel"><h3>分歧仲裁区</h3>${rows ? `<table><thead><tr><th>类型</th><th>片段</th><th>匹配依据</th><th>操作</th></tr></thead><tbody>${rows}</tbody></table>` : '<p class="agreement">当前没有待仲裁分歧。</p>'}</div>`;
}

function qualityPanel(report) {
  if (!report) return '<div class="panel"><h3>质量检查结果</h3><p>尚未执行发布前检查。</p></div>';
  return `<div class="panel"><h3>质量检查结果</h3><p>覆盖 ${report.coveredClips}/${report.clipCount}（${(report.coverageRate * 100).toFixed(0)}%），${report.passed ? '全部规则通过' : '存在阻断项'}。</p>${report.issues.map(issue => `<div class="issue"><strong>${escapeHTML(issue.code)}</strong> · ${escapeHTML(issue.message)}<br><small>片段 ${escapeHTML(issue.clipId || '-')} / 事件 ${escapeHTML(issue.eventId || '-')} / 分歧 ${escapeHTML(issue.disputeId || '-')}</small></div>`).join('')}</div>`;
}

function manifestPanel(batch) {
  const manifest = batch.manifest;
  if (!manifest) return '';
  const enabled = state.viewer.role === 'release_manager' || state.viewer.role === 'reviewer';
  return `<div class="panel"><h3>发布凭据与明细核验</h3>
    <p>清单 ${escapeHTML(manifest.id)} · 事件 ${manifest.normalizedEvents.length} · 发布人 ${escapeHTML(manifest.releasedBy)}</p>
    <div class="manifest">Manifest SHA-256<br>${escapeHTML(manifest.manifestSHA256)}<br><br>片段摘要<br>${escapeHTML(manifest.clipDigest)}<br><br>仲裁摘要<br>${escapeHTML(manifest.adjudicationDigest)}</div>
    <div class="manifest-filter"><label>片段<input id="manifestClip" maxlength="128"></label><label>物种代码<input id="manifestSpecies" maxlength="32"></label><label>开始毫秒<input id="manifestStart" type="number" min="0"></label><label>结束毫秒<input id="manifestEnd" type="number" min="0"></label><label>页码<input id="manifestPage" type="number" min="1" value="1"></label><label>每页<input id="manifestPageSize" type="number" min="1" max="100" value="20"></label></div>
    <div class="toolbar"><button id="loadManifest" ${!enabled ? 'disabled' : ''}>检索明细</button><button id="verifyManifest" ${!enabled ? 'disabled' : ''}>只读核验摘要</button></div>
    <div id="manifestDetails"></div>
  </div>`;
}

function bindActions(batch, clips, queue, submissions, tasks) {
  $('#scope').onclick = () => scopeDialog(batch);
  $('#addClip').onclick = () => clipDialog(batch);
  $('#bulkClips').onclick = () => bulkClipDialog(batch);
  $('#freeze').onclick = () => runWrite(`/api/batches/${batch.id}/freeze`, metadata(batch, 'manager-1', 'manager'));
  $('#check').onclick = () => runWrite(`/api/batches/${batch.id}/quality`, metadata(batch));
  $('#release').onclick = () => runWrite(`/api/batches/${batch.id}/release`, {...metadata(batch), releasedBy: state.viewer.actorId});
  document.querySelectorAll('[data-annotate]').forEach(button => button.onclick = () => annotationDialog(batch, clips.find(clip => clip.id === button.dataset.annotate), tasks));
  document.querySelectorAll('[data-task-edit]').forEach(button => {
    const task = tasks.find(item => item.id === button.dataset.taskEdit);
    button.onclick = () => annotationDialog(batch, clips.find(clip => clip.id === task.clipId), tasks);
  });
  document.querySelectorAll('[data-remove]').forEach(button => button.onclick = () => runWrite(`/api/batches/${batch.id}/clips/${button.dataset.remove}`, metadata(batch, 'manager-1', 'manager'), 'DELETE'));
  document.querySelectorAll('[data-resolve]').forEach(button => button.onclick = () => resolveDialog(batch, queue.find(item => item.id === button.dataset.resolve), submissions));
  if ($('#loadManifest')) $('#loadManifest').onclick = () => loadManifest(batch);
  if ($('#verifyManifest')) $('#verifyManifest').onclick = () => verifyManifest(batch);
}

function metadata(batch, actorId = state.viewer.actorId, role = state.viewer.role) {
  return {actorId, role, expectedVersion: batch.version};
}

async function runWrite(path, body, method = 'POST') {
  try {
    await api(path, {method, headers: headers(), body: JSON.stringify(body)});
    toast('操作已提交');
    await refresh();
  } catch (error) {
    toast(errorText(error));
  }
}

function showDialog(title, fields, onSubmit) {
  $('#editorTitle').textContent = title;
  $('#editorError').hidden = true;
  $('#editorFields').innerHTML = fields;
  const dialog = $('#editor');
  dialog.showModal();
  $('#editorForm').onsubmit = async event => {
    event.preventDefault();
    const submit = $('#editorSubmit');
    submit.disabled = true;
    $('#editorError').hidden = true;
    try {
      await onSubmit(new FormData(event.target));
      dialog.close();
    } catch (error) {
      $('#editorError').textContent = errorText(error);
      $('#editorError').hidden = false;
    } finally {
      submit.disabled = false;
    }
  };
}

function scopeDialog(batch) {
  showDialog('配置审校范围', `<label>标题<input name="title" value="${attr(batch.title)}" required maxlength="120"></label><label>采集地点<input name="site" value="${attr(batch.siteCode)}" required maxlength="64"></label><label>开始时间<input name="start" type="datetime-local" value="${local(new Date(batch.recordingStart))}" required></label><label>结束时间<input name="end" type="datetime-local" value="${local(new Date(batch.recordingEnd))}" required></label><label>允许物种（逗号分隔）<input name="species" value="${attr((batch.allowedSpeciesCodes || []).join(',') || 'BIRD_A,BIRD_B')}" required></label>`, async form => {
    await api(`/api/batches/${batch.id}/scope`, {method: 'PUT', headers: headers(), body: JSON.stringify({...metadata(batch, 'manager-1', 'manager'), title: form.get('title'), siteCode: form.get('site'), recordingStart: new Date(form.get('start')).toISOString(), recordingEnd: new Date(form.get('end')).toISOString(), allowedSpeciesCodes: form.get('species').split(',').map(value => value.trim())})});
    await refresh();
  });
}

function clipDialog(batch) {
  showDialog('登记录音片段', `<label>来源名称<input name="source" required maxlength="200"></label><label>采集时间<input name="captured" type="datetime-local" value="${local(new Date(batch.recordingStart))}" required></label><label>时长（毫秒）<input name="duration" type="number" min="1" required></label><label>SHA-256 内容摘要<input name="hash" pattern="[a-fA-F0-9]{64}" required></label><label>序号<input name="sequence" type="number" min="1" required></label>`, async form => {
    await api(`/api/batches/${batch.id}/clips`, {method: 'POST', headers: headers(), body: JSON.stringify({...metadata(batch, 'manager-1', 'manager'), sourceName: form.get('source'), capturedAt: new Date(form.get('captured')).toISOString(), durationMs: Number(form.get('duration')), contentSHA256: form.get('hash'), sequence: Number(form.get('sequence'))})});
    await refresh();
  });
}

function bulkRow(batch, sequence) {
  return `<tr class="bulk-row"><td><input name="source" maxlength="200" required></td><td><input name="captured" type="datetime-local" value="${local(new Date(batch.recordingStart))}" required></td><td><input name="duration" type="number" min="1" required></td><td><input name="hash" maxlength="64" required></td><td><input name="sequence" type="number" min="1" value="${sequence}" required></td><td><button type="button" data-delete-row>删除</button></td></tr>`;
}

function bulkClipDialog(batch) {
  const first = Math.max(0, ...(batch.clips || []).map(clip => clip.sequence)) + 1;
  showDialog('录音片段批量登记与冲突预检', `<p>一次最多登记 200 条；任一行失败时整批不写入。</p><div class="table-scroll"><table id="bulkTable"><thead><tr><th>来源</th><th>采集时间</th><th>时长 ms</th><th>SHA-256</th><th>序号</th><th></th></tr></thead><tbody>${bulkRow(batch, first)}${bulkRow(batch, first + 1)}${bulkRow(batch, first + 2)}</tbody></table></div><button type="button" id="bulkAddRow">＋ 增加一行</button>`, async () => {
    const rows = [...document.querySelectorAll('.bulk-row')];
    const clips = rows.map(row => ({sourceName: row.querySelector('[name=source]').value, capturedAt: new Date(row.querySelector('[name=captured]').value).toISOString(), durationMs: Number(row.querySelector('[name=duration]').value), contentSHA256: row.querySelector('[name=hash]').value, sequence: Number(row.querySelector('[name=sequence]').value)}));
    await api(`/api/batches/${batch.id}/clips/bulk`, {method: 'POST', headers: headers(), body: JSON.stringify({...metadata(batch, 'manager-1', 'manager'), clips})});
    toast(`已原子登记 ${clips.length} 条片段`);
    await refresh();
  });
  $('#bulkAddRow').onclick = () => {
    const body = $('#bulkTable tbody');
    if (body.children.length >= 200) return;
    body.insertAdjacentHTML('beforeend', bulkRow(batch, first + body.children.length));
    bindDeleteRows();
  };
  bindDeleteRows();
}

function bindDeleteRows() {
  document.querySelectorAll('[data-delete-row]').forEach(button => button.onclick = () => {
    if (document.querySelectorAll('.bulk-row').length > 1) button.closest('tr').remove();
  });
}

function eventRow(event = {}, species = '') {
  return `<tr class="event-row" data-event-id="${attr(event.id || '')}"><td><input name="species" value="${attr(event.speciesCode || species)}" maxlength="32" required></td><td><input name="start" type="number" min="0" value="${event.startMs ?? 0}" required></td><td><input name="end" type="number" min="1" value="${event.endMs ?? 1000}" required></td><td><select name="confidence"><option ${event.confidence === 'high' ? 'selected' : ''}>high</option><option ${event.confidence === 'medium' ? 'selected' : ''}>medium</option><option ${event.confidence === 'low' ? 'selected' : ''}>low</option></select></td><td><textarea name="evidence" maxlength="1000" required>${escapeHTML(event.evidenceNote || '')}</textarea></td><td><button type="button" data-delete-event>删除</button></td></tr>`;
}

async function annotationDialog(batch, clip, tasks) {
  try {
    const query = new URLSearchParams(state.viewer);
    const view = await api(`/api/batches/${batch.id}?${query}`);
    const task = tasks.find(item => item.clipId === clip.id && item.targetAnnotator === state.viewer.actorId && item.status !== 'closed');
    const candidates = view.submissions.filter(item => item.clipId === clip.id && item.annotatorId === state.viewer.actorId);
    const draft = candidates.sort((a, b) => b.round - a.round).find(item => item.status !== 'submitted');
    const round = task?.round || draft?.round || 1;
    const initialEvents = draft?.events?.length ? draft.events : [{}];
    showDialog(`${task ? '返标' : '标注'} ${clip.sourceName} · R${round}`, `<p>${task ? `退回理由：${escapeHTML(task.reason)}<br>原匹配依据：${escapeHTML(task.originalBasis.explanation || '无')}` : '草稿仅对当前标注员可见。'}</p><div class="table-scroll"><table id="eventTable"><thead><tr><th>物种</th><th>开始 ms</th><th>结束 ms</th><th>置信</th><th>证据说明</th><th></th></tr></thead><tbody>${initialEvents.map(event => eventRow(event, batch.allowedSpeciesCodes?.[0] || '')).join('')}</tbody></table></div><button type="button" id="addEvent">＋ 增加事件</button><label>修订说明${round > 1 ? '（返标提交必填）' : ''}<textarea name="revision" ${round > 1 ? 'required' : ''}>${escapeHTML(draft?.revisionReason || '')}</textarea></label><label>操作<select name="intent"><option value="save">仅保存草稿</option><option value="submit">确认并提交本轮</option></select></label><div id="eventSummary" class="summary"></div>`, async form => {
      const events = collectEvents();
      if (form.get('intent') === 'submit') {
        const summary = summarizeEvents(events);
        if (!window.confirm(`提交前复核：${summary}。提交后本轮将锁定，是否确认？`)) throw new Error('已取消提交，当前编辑仍保留');
      }
      const saved = await api(`/api/batches/${batch.id}/clips/${clip.id}/draft`, {method: 'PUT', headers: headers(), body: JSON.stringify({...metadata(batch, state.viewer.actorId, 'annotator'), submissionId: draft?.id || '', annotatorId: state.viewer.actorId, round, revisionReason: form.get('revision'), events})});
      if (form.get('intent') === 'submit') {
        await api(`/api/batches/${batch.id}/clips/${clip.id}/submit`, {method: 'POST', headers: headers(), body: JSON.stringify({actorId: state.viewer.actorId, role: 'annotator', expectedVersion: saved.version, annotatorId: state.viewer.actorId, round, revisionReason: form.get('revision'), confirmed: true})});
        toast('本轮标注已确认提交并锁定');
      } else {
        toast('多事件草稿已保存');
      }
      await refresh();
    });
    $('#addEvent').onclick = () => {
      if (document.querySelectorAll('.event-row').length >= 200) return;
      $('#eventTable tbody').insertAdjacentHTML('beforeend', eventRow({}, batch.allowedSpeciesCodes?.[0] || ''));
      bindEventRows();
      bindEventSummaryInputs();
      updateEventSummary();
    };
    bindEventRows();
    bindEventSummaryInputs();
    updateEventSummary();
  } catch (error) {
    toast(errorText(error));
  }
}

function bindEventRows() {
  document.querySelectorAll('[data-delete-event]').forEach(button => button.onclick = () => {
    button.closest('tr').remove();
    updateEventSummary();
  });
}

function bindEventSummaryInputs() {
  document.querySelectorAll('#eventTable input,#eventTable select,#eventTable textarea').forEach(input => input.oninput = updateEventSummary);
}

function collectEvents() {
  return [...document.querySelectorAll('.event-row')].map(row => ({id: row.dataset.eventId || '', speciesCode: row.querySelector('[name=species]').value, startMs: Number(row.querySelector('[name=start]').value), endMs: Number(row.querySelector('[name=end]').value), confidence: row.querySelector('[name=confidence]').value, evidenceNote: row.querySelector('[name=evidence]').value}));
}

function summarizeEvents(events) {
  if (!events.length) return '0 条事件，无区间';
  return `${events.length} 条事件，区间 ${Math.min(...events.map(event => event.startMs))}–${Math.max(...events.map(event => event.endMs))}ms`;
}

function updateEventSummary() {
  if ($('#eventSummary')) $('#eventSummary').textContent = `提交前摘要：${summarizeEvents(collectEvents())}`;
}

function resolveDialog(batch, dispute, submissions) {
  const annotators = [...new Set(submissions.filter(item => item.clipId === dispute.clipId && item.status === 'submitted').map(item => item.annotatorId))];
  showDialog('处理分歧', `<p>${escapeHTML(dispute.basis.explanation)}</p><label>结论<select name="kind"><option value="accept_left">采纳左方</option><option value="accept_right">采纳右方</option><option value="merge">合并修订</option><option value="no_event">无有效事件</option><option value="return">定向退回返标</option></select></label><label>返标员（必须与本分歧关联）<select name="target">${annotators.map(item => `<option>${escapeHTML(item)}</option>`).join('')}</select></label><label>合并物种<input name="species" value="${attr(batch.allowedSpeciesCodes?.[0] || '')}"></label><label>合并区间（毫秒）<div class="toolbar"><input name="start" type="number" value="100"><input name="end" type="number" value="1000"></div></label><label>合并证据<textarea name="evidence">复核后合并双方时间边界</textarea></label><label>仲裁或返标理由<textarea name="reason" required maxlength="1000">依据声谱结构完成复核</textarea></label>`, async form => {
    const kind = form.get('kind');
    const normalizedEvent = kind === 'merge' ? {speciesCode: form.get('species'), startMs: Number(form.get('start')), endMs: Number(form.get('end')), confidence: 'high', evidenceNote: form.get('evidence')} : null;
    await api(`/api/batches/${batch.id}/disputes/${dispute.id}/resolve`, {method: 'POST', headers: headers(), body: JSON.stringify({...metadata(batch, 'reviewer-1', 'reviewer'), reviewerId: 'reviewer-1', resolution: {kind, returnAnnotator: kind === 'return' ? form.get('target') : '', normalizedEvent, reason: form.get('reason')}})});
    await refresh();
  });
}

async function loadManifest(batch) {
  try {
    const query = new URLSearchParams({...state.viewer, clipId: $('#manifestClip').value, speciesCode: $('#manifestSpecies').value, startMs: $('#manifestStart').value, endMs: $('#manifestEnd').value, page: $('#manifestPage').value, pageSize: $('#manifestPageSize').value});
    const details = await api(`/api/batches/${batch.id}/manifest?${query}`);
    $('#manifestDetails').innerHTML = `<h4>规范化事件（命中 ${details.total}）</h4><table><thead><tr><th>片段</th><th>物种</th><th>区间</th><th>置信</th><th>来源</th></tr></thead><tbody>${details.events.map(event => `<tr><td>${escapeHTML(event.clipId)}</td><td>${escapeHTML(event.speciesCode)}</td><td>${event.startMs}–${event.endMs}ms</td><td>${escapeHTML(event.confidence)}</td><td>${escapeHTML(event.source)}</td></tr>`).join('')}</tbody></table><details><summary>来源片段摘要组成（${details.sourceClips.length}）</summary>${details.sourceClips.map(clip => `<p>${escapeHTML(clip.clipId)} · ${escapeHTML(clip.sourceName)} · ${clip.durationMs}ms · <code>${escapeHTML(clip.contentSHA256)}</code></p>`).join('')}</details><details><summary>仲裁轨迹摘要组成（${details.adjudicationTrail.length}）</summary>${details.adjudicationTrail.map(item => `<p>${escapeHTML(item.disputeId)} · ${escapeHTML(item.kind)} · ${escapeHTML(item.reason)}</p>`).join('') || '<p>无仲裁记录</p>'}</details>`;
  } catch (error) {
    toast(errorText(error));
  }
}

async function verifyManifest(batch) {
  try {
    const query = new URLSearchParams(state.viewer);
    const result = await api(`/api/batches/${batch.id}/manifest/verify?${query}`);
    $('#manifestDetails').innerHTML = `<h4>摘要核验：${result.consistent ? '全部一致' : '存在不一致'}</h4>${result.items.map(item => `<div class="${item.consistent ? 'check-ok' : 'issue'}"><strong>${escapeHTML(item.field)}</strong> · ${item.consistent ? '一致' : '不一致'}<br><small>清单值 ${escapeHTML(item.expected || '缺失')}<br>复算值 ${escapeHTML(item.actual)}</small></div>`).join('')}`;
  } catch (error) {
    toast(errorText(error));
  }
}

function createDialog() {
  const now = new Date();
  const later = new Date(now.getTime() + 3600000);
  showDialog('创建审校批次', `<label>标题<input name="title" required maxlength="120"></label><label>采集地点<input name="site" required maxlength="64"></label><label>开始时间<input name="start" type="datetime-local" value="${local(now)}" required></label><label>结束时间<input name="end" type="datetime-local" value="${local(later)}" required></label>`, async form => {
    await api('/api/batches', {method: 'POST', headers: headers(), body: JSON.stringify({actorId: 'manager-1', role: 'manager', expectedVersion: 0, title: form.get('title'), siteCode: form.get('site'), recordingStart: new Date(form.get('start')).toISOString(), recordingEnd: new Date(form.get('end')).toISOString()})});
    await refresh();
  });
}

const statusName = status => ({draft: '草拟', frozen: '已冻结', annotating: '标注中', adjudicating: '仲裁中', ready: '待发布', released: '已发布', submitted: '已提交', reopened: '待返标', pending: '待领取', in_progress: '返标中', closed: '已闭环'}[status] || status);
const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[char]));
const attr = escapeHTML;
const local = date => new Date(date - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);

$('#editorCancel').onclick = () => $('#editor').close();
$('#newBatch').onclick = createDialog;
$('#identity').onchange = async event => {
  const [actorId, role] = event.target.value.split('|');
  state.viewer = {actorId, role};
  await refresh();
};
refresh();
