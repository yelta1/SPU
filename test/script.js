// Handle help form submission
const helpForm = document.getElementById('helpForm');
const formMessage = document.getElementById('formMessage');

if (helpForm) {
    helpForm.addEventListener('submit', function(e) {
        e.preventDefault();
        formMessage.textContent = 'Спасибо за обращение! Ваша заявка получена.';
        helpForm.reset();
    });
}

// FAQ toggle logic
const faqQuestions = document.querySelectorAll('.faq-question');

faqQuestions.forEach(btn => {
    btn.addEventListener('click', function() {
        const item = btn.parentElement;
        item.classList.toggle('active');
    });
});

// --- API helpers ---
async function api(url, options = {}) {
    const res = await fetch(url, {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
        ...options
    });
    if (!res.ok) {
        let msg = 'Ошибка запроса';
        try { const data = await res.json(); if (data.error) msg = data.error; } catch {}
        throw new Error(msg);
    }
    return res.json();
}

// --- Login logic ---
const loginContainer = document.getElementById('loginContainer');
const loginForm = document.getElementById('loginForm');
const loginMessage = document.getElementById('loginMessage');
const appMain = document.getElementById('appMain');
const userRole = document.getElementById('userRole');
const userDropdown = document.getElementById('userDropdown');
const logoutBtn = document.getElementById('logoutBtn');
const loginPassword = document.getElementById('loginPassword');
const loginLogin = document.getElementById('loginLogin');
const userFio = document.getElementById('userFio');
const userInfo = document.querySelector('.user-info');
let allowedTypes = [];

async function fetchAllowedTypes() {
    try {
        allowedTypes = await api('/api/allowed-types');
    } catch {
        allowedTypes = [];
    }
}

function showApp(user) {
    loginContainer.style.display = 'none';
    appMain.style.display = '';
    userFio.textContent = user.fio;
    userRole.textContent = user.role;
}

function showLogin() {
    loginContainer.style.display = 'flex';
    appMain.style.display = 'none';
    loginForm.reset();
    loginMessage.textContent = '';
}

loginForm.addEventListener('submit', async function(e) {
    e.preventDefault();
    const login = loginLogin.value.trim();
    const password = loginPassword.value;
    if (!login || !password) {
        loginMessage.textContent = 'Пожалуйста, введите логин и пароль.';
        return;
    }
    try {
        const data = await api('/api/login', {
            method: 'POST',
            body: JSON.stringify({ login, password })
        });
        showApp(data);
        await fetchAllowedTypes();
        renderRequests(currentClassKey);
        renderMyRequests();
    } catch (err) {
        loginMessage.textContent = err.message;
    }
});

logoutBtn.addEventListener('click', async function() {
    try {
        await api('/api/logout', { method: 'POST' });
    } catch {}
    showLogin();
});

// Dropdown logic
let dropdownOpen = false;
// Удалить userNameClickable.addEventListener
// userNameClickable.addEventListener('click', function(e) { ... });
userFio.addEventListener('click', function(e) {
    e.stopPropagation();
    userInfo.classList.toggle('show-dropdown');
});
document.addEventListener('click', function() {
    userInfo.classList.remove('show-dropdown');
});

// --- Requests logic ---
async function renderMyRequests() {
    const container = document.getElementById('pageMyRequests');
    if (!container) return;
    let html = `<h2>Мои заявки</h2>`;
    try {
        const reqs = await api('/api/my-requests');
        if (!reqs.length) {
            html += '<p>Заявки не найдены.</p>';
        } else {
            html += `<div class='my-requests-table-wrap'><table class='my-requests-table'><thead><tr><th>ID</th><th>Тема</th><th>Статус</th><th>Дата</th></tr></thead><tbody>`;
            reqs.forEach(req => {
                html += `<tr><td>${req.id}</td><td>${req.title}</td><td>${req.status}</td><td>${req.date}</td></tr>`;
            });
            html += `</tbody></table></div>`;
        }
    } catch (err) {
        html += `<p style='color:red;'>${err.message}</p>`;
    }
    container.innerHTML = html;
}

const navMyRequestsBtn = document.getElementById('navMyRequests');
if (navMyRequestsBtn) {
    navMyRequestsBtn.addEventListener('click', renderMyRequests);
}

// --- Data for requests by classification (for cards only) ---
const requestsData = {
    base: [
        { title: 'Проблемы с принтером', desc: 'Сообщить о проблемах с принтерами университета или запросить обслуживание.' },
        { title: 'Доступ к Wi-Fi', desc: 'Получить помощь с подключением к Wi-Fi в кампусе.' },
        { title: 'Бюро находок', desc: 'Сообщить или узнать о потерянных вещах в кампусе.' }
    ],
    integrated: [
        { title: 'Проблема с доступом к LMS', desc: 'Проблемы со входом в систему дистанционного обучения.' },
        { title: 'Настройка почты', desc: 'Помощь с университетской почтой или интеграцией с другими системами.' }
    ],
    it: [
        { title: 'ИТ-поддержка', desc: 'Помощь с компьютерами, установкой ПО или устранением неполадок.' }
    ],
    incident: [
        { title: 'Сообщить об инциденте безопасности', desc: 'Уведомить ИТ о подозрительной активности или нарушениях безопасности.' },
        { title: 'Сбой системы', desc: 'Сообщить о сбое системы или сервиса.' }
    ]
};

// --- Modal logic for request forms ---
const modalOverlay = document.getElementById('modalOverlay');
const requestModal = document.getElementById('requestModal');
const modalTitle = document.getElementById('modalTitle');
const modalClose = document.getElementById('modalClose');
const requestForm = document.getElementById('requestForm');
const modalMessage = document.getElementById('modalMessage');
let currentRequestTitle = '';
let currentRequestDescr = document.getElementById("requestDetails");

function openModal(title) {
    modalTitle.textContent = title;
    modalMessage.textContent = '';
    requestForm.reset();
    modalOverlay.style.display = 'block';
    requestModal.style.display = 'block';
    currentRequestTitle = title;
}

function closeModal() {
    modalOverlay.style.display = 'none';
    requestModal.style.display = 'none';
    currentRequestTitle = '';
}

modalOverlay.addEventListener('click', closeModal);
modalClose.addEventListener('click', closeModal);
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeModal();
});

function attachCardEvents() {
    document.querySelectorAll('.request-card').forEach(card => {
        card.addEventListener('click', function() {
            const title = card.querySelector('.request-title').textContent;
            openModal(title);
        });
    });
}

let currentClassKey = 'base';
function renderRequests(classKey) {
    currentClassKey = classKey;
    if (!allowedTypes.includes(classKey)) {
        document.getElementById('requests').innerHTML = '<p style="color:gray;">Данный тип заявок вам недоступен</p>';
        return;
    }
    const requestsContainer = document.getElementById('requests');
    requestsContainer.innerHTML = '';
    const reqs = requestsData[classKey] || [];
    reqs.forEach(req => {
        const card = document.createElement('div');
        card.className = 'request-card';
        card.innerHTML = `<div class="request-title">${req.title}</div><div class="request-desc">${req.desc}</div>`;
        requestsContainer.appendChild(card);
    });
    attachCardEvents();
}

requestForm.addEventListener('submit', async function(e) {
    e.preventDefault();
    try {
        await api('/api/new-request', {
            method: 'POST',
            body: JSON.stringify({ 
                title: currentRequestTitle,
                descr: currentRequestDescr
             })
        });
        modalMessage.textContent = `Ваша заявка по теме "${currentRequestTitle}" отправлена!`;
        requestForm.reset();
        renderMyRequests();
    } catch (err) {
        modalMessage.textContent = err.message;
    }
});

const classificationEls = document.querySelectorAll('.classification');
classificationEls.forEach(el => {
    el.addEventListener('click', function() {
        classificationEls.forEach(c => c.classList.remove('active'));
        el.classList.add('active');
        renderRequests(el.getAttribute('data-class'));
    });
});

renderRequests('base');

const navPages = [
    { btn: 'navGeneral', page: 'pageGeneral' },
    { btn: 'navMyRequests', page: 'pageMyRequests' },
    { btn: 'navNewRequest', page: 'pageRequests' },
    { btn: 'navKnowledge', page: 'pageKnowledge' },
    { btn: 'navFAQ', page: 'pageFAQ' }
];

function showPage(pageId) {
    const requestContainer = document.getElementById('requestContainer');
    const mainPages = document.getElementById('mainPages');
    if (pageId === 'pageRequests') {
        requestContainer.style.display = '';
        mainPages.style.display = 'none';
    } else {
        requestContainer.style.display = 'none';
        mainPages.style.display = '';
    }
    navPages.forEach(({ btn, page }) => {
        const btnEl = document.getElementById(btn);
        if (btnEl) btnEl.classList.toggle('active', page === pageId);
    });
    ['pageGeneral','pageMyRequests','pageKnowledge','pageFAQ'].forEach(pid => {
        const el = document.getElementById(pid);
        if (el) el.style.display = (pid === pageId) ? '' : 'none';
    });
    const reqPage = document.getElementById('pageRequests');
    if (reqPage) reqPage.style.display = (pageId === 'pageRequests') ? '' : 'none';
}

navPages.forEach(({ btn, page }) => {
    document.getElementById(btn).addEventListener('click', () => showPage(page));
});

showPage('pageGeneral');

// On load, check session
(async function() {
    try {
        const reqs = await api('/api/my-requests');
        const types = await api('/api/allowed-types');
        allowedTypes = types;
        showApp({ fio: userFio.textContent, role: userRole.textContent });
        renderRequests(currentClassKey);
        renderMyRequests();
    } catch {
        showLogin();
    }
})(); 
