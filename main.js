    const { createApp, ref, computed, onMounted, watch } = Vue;
    const API_URL = '/api'; 

    createApp({
        setup() {
            // --- 1. АВТОРИЗАЦИЯ И ПОЛЬЗОВАТЕЛЬ ---
            const user = ref(null);
            const token = ref(localStorage.getItem('cw_token') || null); // ДОБАВЛЕН ТОКЕН
            const loginForm = ref({ username: '', password: '' });
            const loginError = ref('');
            
            const showProfileModal = ref(false);
            const profileForm = ref({ username: '', password: '', name: '' });

            // --- 2. НАВИГАЦИЯ И ИНТЕРФЕЙС ---
            const tabs = [
                { id: 'home', label: 'Главная', shortLabel: 'Главная', icon: 'fas fa-home', role: 'all' },
                { id: 'reports', label: 'Отчеты', shortLabel: 'Отчеты', icon: 'fas fa-chart-pie', role: 'all' },
                { id: 'services', label: 'Прайс', shortLabel: 'Прайс', icon: 'fas fa-list', role: 'admin' },
                { id: 'expenses', label: 'Расходы', shortLabel: 'Расходы', icon: 'fas fa-receipt', role: 'admin' },
                { id: 'users', label: 'Команда', shortLabel: 'Команда', icon: 'fas fa-users', role: 'admin' }
            ];
            const currentTab = ref(localStorage.getItem('cw_tab') || 'home');
            const canAccess = (tab) => tab.role === 'all' || (user.value && user.value.role === 'admin');

            const toasts = ref([]);
            const confirmState = ref({ show: false, message: '', action: () => {} });
            
            // --- 3. ДАННЫЕ (СОСТОЯНИЕ) ---
            const today = new Date().toISOString().split('T')[0];
            const homeDate = ref(today);
            const expenseDates = ref(JSON.parse(localStorage.getItem('cw_exp_dates') || JSON.stringify({start: today, end: today})));
            const reportDates = ref(JSON.parse(localStorage.getItem('cw_dates') || JSON.stringify({start: today, end: today})));
            const reportFilterId = ref(0);
            
            const washes = ref([]); 
            const expenses = ref([]); 
            const usersList = ref([]); 
            const servicesList = ref([]);
            const bonuses = ref([]); 
            
            const newEntry = ref({ date: today, price: '', carModel: '', description: '', selectedWorkerId: null, coWorkerId: null, is_free: false });
            const newExpense = ref({ date: today, title: '', amount: '' });
            const newBonus = ref({ date: today, user_id: null, amount: '', description: 'Округление смены' });
            
            const selectedServices = ref([]);
            const selectedService = ref(null); 
            const customServiceMode = ref(false);
            const customServiceText = ref('');

            const showServiceModal = ref(false); const editingService = ref({});
            const showUserModal = ref(false); const editingUser = ref({});
            const showWashModal = ref(false); const editingWash = ref({});
            const showExpenseEditModal = ref(false); const editingExpense = ref({});
            const showBonusModal = ref(false);

            // --- 4. ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---
            const showToast = (title, msg, type = 'success') => { const id = Date.now(); toasts.value.push({ id, title, msg, type }); setTimeout(() => toasts.value = toasts.value.filter(t => t.id !== id), 3000); };
            const confirmAction = (msg, action) => { confirmState.value = { show: true, message: msg, action: () => { action(); confirmState.value.show = false; } }; };
            const formatMoney = (v) => new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(v);
            const formatDate = (v) => v ? new Date(v).toLocaleDateString('ru-RU') : '';

            // --- 5. МЕТОДЫ API (ТЕПЕРЬ С ТОКЕНОМ) ---
            const api = async (action, data = {}) => { 
                try { 
                    const headers = { 'Content-Type': 'application/json' };
                    if (token.value) {
                        headers['Authorization'] = `Bearer ${token.value}`;
                    }

                    const res = await fetch(`${API_URL}?action=${action}`, { 
                        method: 'POST', 
                        headers: headers,
                        body: JSON.stringify(data) 
                    }); 

                    // Если бэкенд сказал "Доступ запрещен" (401), выкидываем на окно логина
                    if (res.status === 401) {
                        logout();
                        showToast('Сессия истекла', 'Пожалуйста, войдите снова', 'error');
                        return { success: false };
                    }
                    if (res.status === 403) {
                        showToast('Доступ запрещен', 'У вас нет прав для этого действия', 'error');
                        return { success: false };
                    }

                    return await res.json(); 
                } catch(e) { 
                    showToast('Ошибка сети', 'Нет связи с сервером', 'error'); 
                    return { success: false }; 
                } 
            };

            const loadData = async () => {
                if (!user.value) return; // Не грузим данные, если не вошли
                let start, end;
                if (currentTab.value === 'home') {
                    start = homeDate.value; end = homeDate.value;
                } else if (currentTab.value === 'expenses') {
                    start = expenseDates.value.start; end = expenseDates.value.end;
                } else {
                    start = reportDates.value.start; end = reportDates.value.end;
                }
                
                const res = await api('get_report', { start, end, user_id: user.value.id, role: user.value.role, filter_id: reportFilterId.value });
                if(res && !res.error) {
                    washes.value = res.washes || []; 
                    expenses.value = res.expenses || [];
                    bonuses.value = res.bonuses || []; 
                }
            };

            const loadAux = async () => {
                if (!user.value) return;
                const uRes = await api('get_users');
                if(uRes && !uRes.error) usersList.value = uRes;
                
                const sRes = await api('get_services');
                if(sRes && !sRes.error) servicesList.value = sRes;
            };

            const login = async () => {
                const res = await api('login', loginForm.value);
                if(res.success) { 
                    user.value = res.user; 
                    token.value = res.token; // Сохраняем токен
                    localStorage.setItem('cw_user', JSON.stringify(user.value)); 
                    localStorage.setItem('cw_token', token.value);
                    loadAux(); 
                    loadData(); 
                    showToast('Успешно', `Привет, ${user.value.name}`); 
                } else {
                    loginError.value = res.error || "Неверный логин или пароль";
                }
            };

            const logout = () => { 
                user.value = null; 
                token.value = null;
                localStorage.removeItem('cw_user'); 
                localStorage.removeItem('cw_token');
            };
            
            const saveProfile = async () => {
                const res = await api('update_profile', profileForm.value);
                if(res.success) {
                    user.value.username = profileForm.value.username;
                    localStorage.setItem('cw_user', JSON.stringify(user.value));
                    showProfileModal.value = false;
                    showToast('Готово', 'Профиль обновлен');
                }
            };

            // --- 6. ОБРАБОТЧИКИ И ЛОГИКА ---
            const changeDay = (offset) => { const d = new Date(homeDate.value); d.setDate(d.getDate() + offset); homeDate.value = d.toISOString().split('T')[0]; };
            const changeExpenseDay = (offset) => { 
                const d = new Date(expenseDates.value.start); 
                d.setDate(d.getDate() + offset); 
                const newDateStr = d.toISOString().split('T')[0];
                expenseDates.value.start = newDateStr;
                expenseDates.value.end = newDateStr;
                loadData(); 
            };

            const updateDescription = () => {
                let parts = selectedServices.value.map(s => s.name);
                if (customServiceMode.value && customServiceText.value) parts.push(customServiceText.value);
                newEntry.value.description = parts.join(' + ');
            };
            
            const toggleService = (s) => {
                const idx = selectedServices.value.findIndex(x => x.id === s.id);
                if (idx > -1) selectedServices.value.splice(idx, 1);
                else selectedServices.value.push(s);
                newEntry.value.price = selectedServices.value.reduce((sum, item) => sum + item.price, 0);
                updateDescription();
            };

            const toggleCustomService = () => {
                customServiceMode.value = !customServiceMode.value;
                if(!customServiceMode.value) customServiceText.value = '';
                updateDescription();
            };
            
            const onServiceChange = () => { if(selectedService.value === 'custom') { toggleCustomService(); selectedService.value = null; } };

            const addWash = async () => {
                if(!newEntry.value.price || !newEntry.value.description) return;
                const payload = {
                    user_id: newEntry.value.selectedWorkerId || user.value.id,
                    co_worker_id: newEntry.value.coWorkerId || null,
                    date: newEntry.value.date,
                    car_model: newEntry.value.carModel,
                    description: newEntry.value.description,
                    price: newEntry.value.price,
                    is_free: newEntry.value.is_free ? 1 : 0
                };
                await api('add_wash', payload);
                newEntry.value.price = ''; newEntry.value.carModel = ''; newEntry.value.description = ''; 
                newEntry.value.coWorkerId = null; newEntry.value.is_free = false;
                selectedServices.value = []; customServiceMode.value = false; customServiceText.value = '';
                showToast('Сохранено', 'Машина добавлена'); loadData();
            };

            const saveWash = async () => { await api('edit_wash', { ...editingWash.value, date: editingWash.value.date_only, is_free: editingWash.value.is_free ? 1 : 0 }); showWashModal.value = false; showToast('Обновлено', 'Запись изменена'); loadData(); };
            const deleteWash = (id) => confirmAction('Удалить эту запись безвозвратно?', async () => { await api('delete_wash', { id }); loadData(); });
            const saveService = async () => { await api('save_service', { id: editingService.value.id, service_name: editingService.value.name, service_price: editingService.value.price }); showServiceModal.value = false; loadAux(); };
            const deleteService = (id) => confirmAction('Архивировать эту услугу?', async () => { await api('delete_service', { id }); loadAux(); });
            const saveUser = async () => { await api('save_user', editingUser.value); showUserModal.value = false; loadAux(); };
            const deleteUser = (id) => confirmAction('Архивировать сотрудника?', async () => { await api('delete_user', { id }); loadAux(); });
            
            const addExpense = async () => { 
                if(!newExpense.value.amount) return; 
                await api('add_expense', { date: newExpense.value.date || today, title: newExpense.value.title || 'Прочий расход', amount: Number(newExpense.value.amount) }); 
                newExpense.value.amount = ''; newExpense.value.title = ''; 
                showToast('Расход', 'Записано'); loadData(); 
            };

            const saveExpenseEdit = async () => {
                if(!editingExpense.value.amount || !editingExpense.value.title) return;
                await api('edit_expense', { id: editingExpense.value.id, title: editingExpense.value.title, amount: Number(editingExpense.value.amount), date: editingExpense.value.date_only });
                showExpenseEditModal.value = false; showToast('Обновлено', 'Расход изменен'); loadData();
            };
            const deleteExpense = (id) => { confirmAction('Удалить этот расход?', async () => { await api('delete_expense', { id }); loadData(); }); };

            const openBonusModal = () => {
                newBonus.value.date = currentTab.value === 'reports' ? reportDates.value.end : today;
                newBonus.value.amount = '';
                newBonus.value.description = 'Округление смены';
                if(selectableWorkers.value.length > 0) newBonus.value.user_id = selectableWorkers.value[0].id;
                showBonusModal.value = true;
            };

            const addBonus = async () => {
                if(!newBonus.value.amount) { showToast('Внимание', 'Введите сумму', 'error'); return; }
                if(!newBonus.value.user_id) { showToast('Внимание', 'Выберите сотрудника', 'error'); return; }
                
                const res = await api('add_bonus', {
                    user_id: newBonus.value.user_id, amount: Number(newBonus.value.amount), date: newBonus.value.date, description: newBonus.value.description || 'Округление'
                });
                
                if(!res.success) { showToast('Ошибка базы', res.error || 'Не удалось начислить', 'error'); return; }
                
                newBonus.value.amount = ''; newBonus.value.description = 'Округление смены';
                showBonusModal.value = false; showToast('Начислено', 'Сумма добавлена к ЗП'); loadData();
            };
            const deleteBonus = (id) => { confirmAction('Удалить это начисление?', async () => { await api('delete_bonus', { id }); loadData(); }); };

            // --- 7. ВЫЧИСЛЯЕМЫЕ СВОЙСТВА (СТАТИСТИКА И ДАТЫ) ---
            const selectableWorkers = computed(() => usersList.value.filter(u => u.role === 'worker' || u.id === user.value.id));
            const fancyDateLabel = computed(() => { if(homeDate.value === today) return 'Сегодня'; const y = new Date(); y.setDate(y.getDate() - 1); if(homeDate.value === y.toISOString().split('T')[0]) return 'Вчера'; return homeDate.value.split('-').reverse().join('.'); });
            
            const fancyExpenseDateLabel = computed(() => { 
                if(expenseDates.value.start !== expenseDates.value.end) return 'Период';
                if(expenseDates.value.start === today) return 'Сегодня'; 
                const y = new Date(); y.setDate(y.getDate() - 1); 
                if(expenseDates.value.start === y.toISOString().split('T')[0]) return 'Вчера'; 
                return expenseDates.value.start.split('-').reverse().join('.'); 
            });

            const expensesTotal = computed(() => expenses.value.reduce((sum, item) => sum + (Number(item.amount) || 0), 0));

            const stats = computed(() => {
                let startD, endD;
                if (currentTab.value === 'home') {
                    startD = new Date(homeDate.value); endD = new Date(homeDate.value);
                } else if (currentTab.value === 'expenses') {
                    startD = new Date(expenseDates.value.start); endD = new Date(expenseDates.value.end);
                } else {
                    startD = new Date(reportDates.value.start); endD = new Date(reportDates.value.end);
                }
                const periodDays = Math.max(1, Math.ceil((endD - startD) / (1000 * 60 * 60 * 24)) + 1);

                let actualRevenue = 0;
                let totalPayroll = 0;
                const workerStats = {}; 
                let totalCars = washes.value.length; 
                
                washes.value.forEach(w => {
                    const basePrice = Number(w.price) || 0;
                    const isFree = w.is_free === 1 || w.is_free === true; 
                    
                    const rev = isFree ? 0 : basePrice;
                    actualRevenue += rev;
                    
                    const numWorkers = w.co_worker_name ? 2 : 1;
                    const revPerWorker = rev / numWorkers;
                    const workerShare = (basePrice / 2) / numWorkers;
                    const ownerShare = revPerWorker - workerShare;
                    
                    if (!workerStats[w.worker_name]) workerStats[w.worker_name] = { earnings: 0, cars: 0, revenue: 0, ownerShare: 0 };
                    workerStats[w.worker_name].earnings += workerShare;
                    workerStats[w.worker_name].revenue += revPerWorker;
                    workerStats[w.worker_name].ownerShare += ownerShare;
                    workerStats[w.worker_name].cars += 1;
                    
                    if (w.co_worker_name) {
                        if (!workerStats[w.co_worker_name]) workerStats[w.co_worker_name] = { earnings: 0, cars: 0, revenue: 0, ownerShare: 0 };
                        workerStats[w.co_worker_name].earnings += workerShare;
                        workerStats[w.co_worker_name].revenue += revPerWorker;
                        workerStats[w.co_worker_name].ownerShare += ownerShare;
                        workerStats[w.co_worker_name].cars += 1;
                    }
                    totalPayroll += (basePrice / 2);
                });
                
                let workerBonusesTotal = 0;
                bonuses.value.forEach(b => {
                    const bAmt = Number(b.amount) || 0;
                    if (!workerStats[b.user_name]) workerStats[b.user_name] = { earnings: 0, cars: 0, revenue: 0, ownerShare: 0 };
                    workerStats[b.user_name].earnings += bAmt;
                    workerStats[b.user_name].ownerShare -= bAmt; 
                    totalPayroll += bAmt; 
                    
                    if (user.value && b.user_id === user.value.id) {
                        workerBonusesTotal += bAmt;
                    }
                });

                const exp = expenses.value.reduce((a, b) => a + (Number(b.amount) || 0), 0);
                
                let workerEarnings = 0;
                let myCars = 0; 
                if (user.value) {
                     workerEarnings = washes.value.reduce((sum, w) => {
                        if (w.user_id === user.value.id || w.co_worker_id === user.value.id) {
                            myCars++;
                            return sum + (w.co_worker_name ? (Number(w.price)||0) / 4 : (Number(w.price)||0) / 2);
                        }
                        return sum;
                    }, 0);
                    workerEarnings += workerBonusesTotal; 
                }

                return { 
                    periodDays, totalCars, myCars, totalRevenue: actualRevenue, totalPayroll: totalPayroll,
                    totalExpenses: exp, netProfit: actualRevenue - totalPayroll - exp, 
                    myEarnings: user.value && user.value.role === 'admin' ? (actualRevenue - totalPayroll) : workerEarnings, 
                    workerStats: workerStats 
                };
            });

            // --- 8. ОТКРЫТИЕ МОДАЛОК ---
            const openProfileModal = () => { profileForm.value = { id: user.value.id, username: user.value.username, password: '', name: user.value.name }; showProfileModal.value = true; };
            const openServiceModal = (s=null) => { editingService.value = s ? {...s} : {}; showServiceModal.value = true; };
            const openUserModal = (u=null) => { editingUser.value = u ? {...u, password:''} : {role:'worker'}; showUserModal.value = true; };
            const openExpenseEditModal = (e) => { editingExpense.value = { ...e }; showExpenseEditModal.value = true; };
            const openWashModal = (w) => { editingWash.value = {id: w.id, user_id: w.user_id, co_worker_id: w.co_worker_id, date_only: w.date_only, car_model: w.car_model, description: w.description, price: w.price, is_free: w.is_free === 1}; showWashModal.value = true; };

            // --- 9. ЖИЗНЕННЫЙ ЦИКЛ (ХУКИ И СЛЕДОПЫТЫ) ---
            onMounted(() => { 
                const u = localStorage.getItem('cw_user'); 
                const t = localStorage.getItem('cw_token');
                if(u && t) { 
                    user.value = JSON.parse(u); 
                    token.value = t;
                    loadAux(); 
                    loadData(); 
                } 
            });
            
            watch(currentTab, (v) => { localStorage.setItem('cw_tab', v); loadData(); });
            watch(homeDate, () => { if(currentTab.value==='home') { newEntry.value.date = homeDate.value; loadData(); } });
            watch(expenseDates, () => { localStorage.setItem('cw_exp_dates', JSON.stringify(expenseDates.value)); }, {deep: true});
            watch([reportDates, reportFilterId], () => { if(currentTab.value==='reports') { localStorage.setItem('cw_dates', JSON.stringify(reportDates.value)); loadData(); } }, {deep:true});

            return { 
                user, loginForm, loginError, login, logout, currentTab, tabs, canAccess, loadData,
                washes, expenses, bonuses, usersList, servicesList, newEntry, newExpense, newBonus, selectedService,
                homeDate, expenseDates, reportDates, reportFilterId, stats, expensesTotal, formatMoney, formatDate,
                showServiceModal, editingService, openServiceModal, saveService, deleteService,
                showUserModal, editingUser, openUserModal, saveUser, deleteUser,
                showWashModal, editingWash, openWashModal, saveWash, deleteWash,
                showProfileModal, profileForm, openProfileModal, saveProfile,
                showExpenseEditModal, editingExpense, openExpenseEditModal, saveExpenseEdit, deleteExpense,
                showBonusModal, openBonusModal, addBonus, deleteBonus,
                addWash, addExpense, onServiceChange, toasts, confirmState,
                changeDay, changeExpenseDay, fancyDateLabel, fancyExpenseDateLabel, selectableWorkers,
                selectedServices, toggleService, customServiceMode, customServiceText, toggleCustomService, updateDescription
            };
        }
    }).mount('#app');
