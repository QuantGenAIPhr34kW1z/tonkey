// tonkey Admin Dashboard JavaScript

// Refresh stats every 30 seconds
let refreshInterval;

document.addEventListener('DOMContentLoaded', function() {
    // Start auto-refresh
    startAutoRefresh();

    // Initialize tooltips and other UI elements
    initUI();
});

function startAutoRefresh() {
    refreshInterval = setInterval(function() {
        refreshStats();
    }, 30000);
}

function stopAutoRefresh() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
}

function refreshStats() {
    const indicator = document.getElementById('refresh-indicator');
    if (indicator) {
        indicator.classList.add('refreshing');
    }

    fetch('/admin/dashboard/api/stats')
        .then(response => response.json())
        .then(data => {
            updateStatsDisplay(data);
            if (indicator) {
                indicator.classList.remove('refreshing');
            }
        })
        .catch(error => {
            console.error('Failed to refresh stats:', error);
            if (indicator) {
                indicator.classList.remove('refreshing');
            }
        });
}

function updateStatsDisplay(stats) {
    // Update stat cards if they exist
    const statElements = {
        'total_users': stats.total_users,
        'total_transactions': stats.total_transactions,
        'active_webhooks': stats.active_webhooks,
        'pending_jobs': stats.pending_jobs
    };

    for (const [key, value] of Object.entries(statElements)) {
        const el = document.querySelector(`[data-stat="${key}"]`);
        if (el) {
            el.textContent = formatNumber(value);
        }
    }
}

function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

function formatAmount(nanoTON) {
    const ton = nanoTON / 1e9;
    return ton.toFixed(2) + ' TON';
}

function initUI() {
    // Initialize any UI components
}

// User Management
function showCreateUserModal() {
    alert('Create User modal - implement with your preferred modal library');
}

function editUser(id) {
    console.log('Edit user:', id);
    window.location.href = '/admin/dashboard/users/' + id;
}

function deleteUser(id) {
    if (confirm('Are you sure you want to delete this user?')) {
        fetch('/admin/users/' + id, {
            method: 'DELETE',
            headers: {
                'Authorization': 'Bearer ' + getToken()
            }
        })
        .then(response => {
            if (response.ok) {
                location.reload();
            } else {
                alert('Failed to delete user');
            }
        });
    }
}

// Webhook Management
function showCreateWebhookModal() {
    alert('Create Webhook modal - implement with your preferred modal library');
}

function testWebhook(id) {
    fetch('/v1/webhooks/' + id + '/test', {
        method: 'POST',
        headers: {
            'Authorization': 'Bearer ' + getToken()
        }
    })
    .then(response => response.json())
    .then(data => {
        alert('Webhook test ' + (data.success ? 'succeeded' : 'failed'));
    });
}

function deleteWebhook(id) {
    if (confirm('Are you sure you want to delete this webhook?')) {
        fetch('/v1/webhooks/' + id, {
            method: 'DELETE',
            headers: {
                'Authorization': 'Bearer ' + getToken()
            }
        })
        .then(response => {
            if (response.ok) {
                location.reload();
            } else {
                alert('Failed to delete webhook');
            }
        });
    }
}

// Transaction Filtering
function filterTransactions(status) {
    const url = new URL(window.location);
    if (status) {
        url.searchParams.set('status', status);
    } else {
        url.searchParams.delete('status');
    }
    window.location = url;
}

// Audit Log
function filterAudit(action) {
    const url = new URL(window.location);
    if (action) {
        url.searchParams.set('action', action);
    } else {
        url.searchParams.delete('action');
    }
    window.location = url;
}

function exportAuditLog() {
    window.location.href = '/admin/dashboard/api/audit/export?format=csv';
}

// Job Management
function filterJobs(status) {
    const url = new URL(window.location);
    if (status) {
        url.searchParams.set('status', status);
    } else {
        url.searchParams.delete('status');
    }
    window.location = url;
}

function retryJob(id) {
    fetch('/admin/jobs/' + id + '/retry', {
        method: 'POST',
        headers: {
            'Authorization': 'Bearer ' + getToken()
        }
    })
    .then(response => {
        if (response.ok) {
            location.reload();
        } else {
            alert('Failed to retry job');
        }
    });
}

function viewJob(id) {
    window.location.href = '/admin/dashboard/jobs/' + id;
}

// Settings
function runMigrations() {
    if (confirm('Run database migrations?')) {
        alert('Migrations would run here');
    }
}

function cleanupAuditLogs() {
    if (confirm('Clean up audit logs older than 90 days?')) {
        alert('Cleanup would run here');
    }
}

function clearJobQueue() {
    if (confirm('Clear all failed jobs?')) {
        alert('Job cleanup would run here');
    }
}

function resetRateLimits() {
    if (confirm('Reset all rate limits? This will allow temporarily blocked IPs to access the system.')) {
        alert('Rate limits would reset here');
    }
}

// Utility
function getToken() {
    // In a real app, get this from a cookie or local storage
    return localStorage.getItem('tonkey_admin_token') || '';
}

// Copy to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        console.log('Copied to clipboard');
    }).catch(function(err) {
        console.error('Failed to copy:', err);
    });
}
