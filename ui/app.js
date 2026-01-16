// Terraform Cost UI - Enhanced Explainability
// UI is READ-ONLY - never computes or infers costs
// ALL values come directly from API response

const API_BASE = '/api/v1';

// State (immutable from API)
let estimateData = null;
let selectedNode = null;

// Navigation
document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.addEventListener('click', () => switchView(btn.dataset.view));
});

function switchView(view) {
    document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
    document.querySelector(`[data-view="${view}"]`)?.classList.add('active');
    document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
    document.getElementById(`${view}-view`)?.classList.add('active');
}

// Main render function - displays API data, never recomputes
function renderEstimate(data) {
    estimateData = data;

    // Metadata
    document.getElementById('engine-version').textContent = `Engine ${data.metadata?.engine_version || '--'}`;
    document.getElementById('footer-hash').textContent = data.metadata?.input_hash?.slice(0, 12) || '--';
    document.getElementById('footer-pricing').textContent = data.metadata?.pricing_snapshot || '--';
    document.getElementById('footer-duration').textContent = `${data.metadata?.duration_ms || 0}ms`;

    // Summary - EXACT values from API
    document.getElementById('total-cost').textContent = formatCost(data.summary?.total_monthly_cost);
    document.getElementById('total-confidence').textContent = formatConfidence(data.summary?.confidence);
    document.getElementById('total-confidence').className = `confidence-value ${data.summary?.confidence_level || ''}`;

    const confBar = document.getElementById('confidence-bar');
    confBar.style.width = `${(data.summary?.confidence || 0) * 100}%`;
    confBar.className = `confidence-bar ${data.summary?.confidence_level || ''}`;

    document.getElementById('confidence-reason').textContent = data.summary?.confidence_reason || '';
    document.getElementById('resource-count').textContent = data.summary?.resource_count || 0;
    document.getElementById('symbolic-stat-count').textContent = data.summary?.symbolic_count || 0;

    // Symbolic banner
    const symbolicCount = data.summary?.symbolic_count || 0;
    const banner = document.getElementById('symbolic-banner');
    banner.style.display = symbolicCount > 0 ? 'flex' : 'none';
    document.getElementById('symbolic-count').textContent = symbolicCount;

    // Cost graph tree
    renderCostGraph(data.costs || []);

    // Symbolic costs section
    renderSymbolicCosts(data.symbolic || []);

    // Confidence contributors
    renderConfidenceContributors(data.costs || []);
}

// Render cost graph as expandable tree
function renderCostGraph(costs) {
    const container = document.getElementById('cost-graph');

    // Group by module (first part of address)
    const modules = groupByModule(costs);

    container.innerHTML = Object.entries(modules).map(([moduleName, resources]) => {
        const moduleConf = getMinConfidence(resources);
        const moduleCost = sumCostsForDisplay(resources);
        const hasSymbolic = resources.some(r => r.is_symbolic);

        return `
            <div class="tree-module ${hasSymbolic ? 'has-symbolic' : ''}">
                <div class="tree-node module-node" onclick="toggleNode(this)">
                    <span class="tree-expand">▶</span>
                    <span class="tree-icon">📦</span>
                    <span class="tree-label">${moduleName}</span>
                    ${hasSymbolic ? '<span class="symbolic-indicator">⚠️</span>' : ''}
                    <div class="tree-right">
                        <span class="tree-cost">${formatCost(moduleCost)}</span>
                        <span class="confidence-badge ${getConfidenceClass(moduleConf)}" 
                              title="Minimum confidence of child resources">${formatConfidenceShort(moduleConf)}</span>
                    </div>
                </div>
                <div class="tree-children">
                    ${resources.map(r => renderResourceNode(r)).join('')}
                </div>
            </div>
        `;
    }).join('');
}

// Render single resource node with confidence tooltip
function renderResourceNode(resource) {
    const isSymbolic = resource.is_symbolic;
    const lineageData = JSON.stringify(resource.lineage || {}).replace(/"/g, '&quot;');

    return `
        <div class="tree-node resource-node ${isSymbolic ? 'symbolic-node' : ''}" 
             onclick="showLineage('${resource.address}', ${lineageData})"
             data-address="${resource.address}">
            <span class="tree-icon">${getResourceIcon(resource.type)}</span>
            <span class="tree-label">${resource.address}</span>
            <span class="tree-type">${resource.type}</span>
            ${isSymbolic ? '<span class="symbolic-tag">UNKNOWN</span>' : ''}
            <div class="tree-right">
                <span class="tree-cost ${isSymbolic ? 'symbolic-cost' : ''}">${isSymbolic ? 'Unknown' : formatCost(resource.monthly_cost)}</span>
                <span class="confidence-badge ${getConfidenceClass(resource.confidence)}"
                      title="Confidence: ${formatConfidence(resource.confidence)}${resource.lineage?.explanation ? ' - ' + resource.lineage.explanation : ''}"
                >${formatConfidenceShort(resource.confidence)}</span>
                <button class="lineage-btn" title="View dependency lineage">🔗</button>
            </div>
        </div>
        ${resource.components?.length ? renderComponents(resource.components) : ''}
    `;
}

// Render cost components
function renderComponents(components) {
    return `
        <div class="tree-components">
            ${components.map(c => `
                <div class="tree-component">
                    <span class="component-name">${c.name}</span>
                    <span class="component-quantity">${c.quantity || ''} ${c.unit || ''}</span>
                    <span class="component-cost">${formatCost(c.monthly_cost)}</span>
                </div>
            `).join('')}
        </div>
    `;
}

// Show lineage in side panel - NEVER recomputes, only displays
function showLineage(address, lineage) {
    selectedNode = address;

    // Highlight selected node
    document.querySelectorAll('.tree-node').forEach(n => n.classList.remove('selected'));
    document.querySelector(`[data-address="${address}"]`)?.classList.add('selected');

    const container = document.getElementById('lineage-content');

    if (!lineage || !lineage.dependency_path?.length) {
        container.innerHTML = `
            <div class="lineage-resource">${address}</div>
            <div class="lineage-message">No dependency information available</div>
        `;
        return;
    }

    container.innerHTML = `
        <div class="lineage-resource">${address}</div>
        <div class="lineage-section">
            <h4>This cost exists because:</h4>
            <div class="lineage-path">
                ${lineage.dependency_path.map((step, i) => `
                    <div class="lineage-step">
                        <span class="lineage-arrow">${i === 0 ? '🏠' : '↓'}</span>
                        <span class="lineage-step-name">${step}</span>
                    </div>
                `).join('')}
            </div>
            <div class="lineage-explanation">
                ${lineage.explanation || 'Direct resource cost'}
            </div>
        </div>
    `;
}

// Render symbolic costs - PROMINENTLY displayed, never hidden
function renderSymbolicCosts(symbolics) {
    const container = document.getElementById('symbolic-costs');
    const section = document.getElementById('symbolic-section');

    if (!symbolics?.length) {
        section.style.display = 'none';
        return;
    }

    section.style.display = 'block';

    container.innerHTML = symbolics.map(s => `
        <div class="symbolic-cost-item">
            <div class="symbolic-item-header">
                <span class="symbolic-item-icon">⚠️</span>
                <span class="symbolic-item-address">${s.address}</span>
            </div>
            <div class="symbolic-item-reason">${s.reason}</div>
            ${s.expression ? `<code class="symbolic-item-expr">${s.expression}</code>` : ''}
            <div class="symbolic-item-bounds">
                ${s.is_unbounded
            ? '<span class="unbounded-warning">⚡ No upper bound estimable</span>'
            : `${s.lower_bound ? `Lower: ${s.lower_bound}` : ''} ${s.upper_bound ? `Upper: ${s.upper_bound}` : ''}`
        }
            </div>
        </div>
    `).join('');
}

// Render confidence contributors - shows lowest confidence resources
function renderConfidenceContributors(costs) {
    const container = document.getElementById('confidence-contributors');

    // Sort by confidence (ascending) - show worst first
    const sorted = [...costs].sort((a, b) => (a.confidence || 1) - (b.confidence || 1));
    const topContributors = sorted.slice(0, 5);

    if (topContributors.length === 0) {
        container.innerHTML = '<div class="empty-state">No confidence data</div>';
        return;
    }

    container.innerHTML = topContributors.map(r => `
        <div class="contributor-item ${r.is_symbolic ? 'symbolic' : ''}"
             onclick="showLineage('${r.address}', ${JSON.stringify(r.lineage || {}).replace(/"/g, '&quot;')})">
            <div class="contributor-address">${r.address}</div>
            <div class="contributor-bar">
                <div class="contributor-fill ${getConfidenceClass(r.confidence)}" 
                     style="width: ${(r.confidence || 0) * 100}%"></div>
            </div>
            <div class="contributor-value">${formatConfidenceShort(r.confidence)}</div>
        </div>
    `).join('');
}

// Toggle tree node expansion
function toggleNode(header) {
    const module = header.parentElement;
    module.classList.toggle('expanded');
    header.querySelector('.tree-expand').textContent = module.classList.contains('expanded') ? '▼' : '▶';
}

function expandAll() {
    document.querySelectorAll('.tree-module').forEach(m => {
        m.classList.add('expanded');
        m.querySelector('.tree-expand').textContent = '▼';
    });
}

function collapseAll() {
    document.querySelectorAll('.tree-module').forEach(m => {
        m.classList.remove('expanded');
        m.querySelector('.tree-expand').textContent = '▶';
    });
}

function scrollToSymbolic() {
    document.getElementById('symbolic-section').scrollIntoView({ behavior: 'smooth' });
}

// Helpers - FORMATTING ONLY, never computation

function formatCost(cost) {
    if (!cost) return '$0.00';
    const num = parseFloat(cost);
    return `$${num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function formatConfidence(c) {
    if (c === undefined || c === null) return '--';
    return `${Math.round(c * 100)}%`;
}

function formatConfidenceShort(c) {
    if (c === undefined || c === null) return '--';
    return `${Math.round(c * 100)}%`;
}

function getConfidenceClass(c) {
    if (c >= 0.9) return 'high';
    if (c >= 0.7) return 'medium';
    if (c >= 0.5) return 'low';
    return 'unknown';
}

function getResourceIcon(type) {
    const icons = {
        'aws_instance': '🖥️', 'aws_db_instance': '🗄️', 'aws_s3_bucket': '🪣',
        'aws_lambda_function': 'λ', 'aws_lb': '⚖️', 'aws_ebs_volume': '💾',
        'aws_nat_gateway': '🌐', 'aws_rds_cluster': '🗄️', 'aws_elasticache': '⚡',
    };
    return icons[type] || '📄';
}

function groupByModule(costs) {
    const modules = {};
    costs.forEach(c => {
        const parts = c.address.split('.');
        const moduleName = parts.length > 2 ? parts.slice(0, -2).join('.') : 'root';
        if (!modules[moduleName]) modules[moduleName] = [];
        modules[moduleName].push(c);
    });
    return modules;
}

// These are for DISPLAY GROUPING only - not recomputation
function getMinConfidence(resources) {
    return Math.min(...resources.map(r => r.confidence || 1));
}

function sumCostsForDisplay(resources) {
    // Note: This sums for UI display grouping only
    // The actual total comes from API
    return resources.reduce((sum, r) => {
        if (r.is_symbolic) return sum;
        return sum + parseFloat(r.monthly_cost || 0);
    }, 0).toFixed(2);
}

// Load demo data
function loadDemo() {
    renderEstimate({
        metadata: {
            input_hash: 'a1b2c3d4e5f6',
            engine_version: '1.0.0',
            pricing_snapshot: 'aws-us-east-1-2026-01-16',
            duration_ms: 412
        },
        summary: {
            total_monthly_cost: '1234.56',
            confidence: 0.72,
            confidence_level: 'medium',
            confidence_reason: 'Unknown usage for aws_lambda_function.api',
            resource_count: 5,
            symbolic_count: 1
        },
        costs: [
            {
                address: 'module.compute.aws_instance.web',
                type: 'aws_instance',
                monthly_cost: '730.00',
                confidence: 0.95,
                lineage: {
                    dependency_path: ['root', 'module.compute', 'aws_instance.web'],
                    explanation: 'EC2 instance t3.xlarge with 730 hours/month'
                },
                components: [
                    { name: 'Compute', monthly_cost: '600.00', quantity: '730', unit: 'hours' },
                    { name: 'EBS Storage', monthly_cost: '130.00', quantity: '100', unit: 'GB' }
                ]
            },
            {
                address: 'module.database.aws_db_instance.main',
                type: 'aws_db_instance',
                monthly_cost: '504.56',
                confidence: 0.72,
                lineage: {
                    dependency_path: ['root', 'module.database', 'aws_db_instance.main'],
                    explanation: 'RDS db.r5.large with assumed 100GB storage'
                }
            },
            {
                address: 'module.workers.aws_instance.worker',
                type: 'aws_instance',
                monthly_cost: '0.00',
                confidence: 0.0,
                is_symbolic: true,
                lineage: {
                    dependency_path: ['root', 'module.config', 'module.workers', 'aws_instance.worker'],
                    explanation: 'Cardinality unknown - depends on module.config.worker_names'
                }
            }
        ],
        symbolic: [
            {
                address: 'module.workers.aws_instance.worker',
                reason: 'for_each derived from module output - cardinality unknown pre-apply',
                expression: 'for_each = module.config.worker_names',
                is_unbounded: true
            }
        ]
    });
}

document.addEventListener('DOMContentLoaded', loadDemo);
