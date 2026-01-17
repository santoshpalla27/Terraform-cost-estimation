import { useState, useEffect } from 'react'
import {
    Header,
    CostSummaryCard,
    CostTree,
    SymbolicExplanationPanel,
    UnsupportedWarning,
    DiffSummary
} from './components'
import type { EstimationResult, ResourceCost, DiffResult } from './types'

function App() {
    const [result, setResult] = useState<EstimationResult | null>(null)
    const [selectedSymbolic, setSelectedSymbolic] = useState<ResourceCost | null>(null)
    const [view, setView] = useState<'estimate' | 'diff'>('estimate')
    const [diff] = useState<DiffResult | null>(null)

    // Demo data
    useEffect(() => {
        setResult({
            totalCost: 12483.50,
            confidence: 0.87,
            coverage: {
                numericPercent: 78,
                symbolicPercent: 15,
                indirectPercent: 2,
                unsupportedPercent: 5,
            },
            resources: [
                {
                    address: 'aws_instance.web',
                    type: 'aws_instance',
                    cost: 876.00,
                    confidence: 0.95,
                    coverageType: 'numeric',
                    components: [
                        { name: 'Compute', category: 't3.large', cost: 60.74, unit: 'hours', quantity: 730, rate: 0.0832, isSymbolic: false },
                        { name: 'Storage', category: 'gp3 100GB', cost: 8.10, unit: 'GB-month', quantity: 100, rate: 0.08, isSymbolic: false },
                    ]
                },
                {
                    address: 'aws_rds_cluster.primary',
                    type: 'aws_rds_cluster',
                    cost: 1250.00,
                    confidence: 0.92,
                    coverageType: 'numeric',
                    components: [
                        { name: 'Instance', category: 'db.r5.large', cost: 175.20, unit: 'hours', quantity: 730, rate: 0.24, isSymbolic: false },
                        { name: 'Storage', category: 'Aurora I/O', cost: 50.00, unit: 'GB-month', quantity: 500, rate: 0.10, isSymbolic: false },
                    ]
                },
                {
                    address: 'aws_lambda_function.api',
                    type: 'aws_lambda_function',
                    cost: 45.00,
                    confidence: 0.50,
                    coverageType: 'symbolic',
                    components: [
                        { name: 'Requests', category: 'Invocations', cost: 20.00, unit: 'requests', quantity: 100000000, rate: 0.0000002, isSymbolic: true, symbolicReason: 'Usage not provided' },
                        { name: 'Duration', category: 'GB-seconds', cost: 25.00, unit: 'GB-seconds', quantity: 1500000, rate: 0.0000166667, isSymbolic: true, symbolicReason: 'Duration estimate' },
                    ]
                },
                {
                    address: 'aws_eks_cluster.main',
                    type: 'aws_eks_cluster',
                    cost: 73.00,
                    confidence: 1.0,
                    coverageType: 'numeric'
                },
                {
                    address: 'aws_eks_node_group.workers',
                    type: 'aws_eks_node_group',
                    cost: 312.00,
                    confidence: 0.88,
                    coverageType: 'numeric'
                },
                {
                    address: 'aws_s3_bucket.assets',
                    type: 'aws_s3_bucket',
                    cost: 120.00,
                    confidence: 0.60,
                    coverageType: 'symbolic'
                },
                {
                    address: 'aws_vpc.main',
                    type: 'aws_vpc',
                    cost: 0,
                    confidence: 1.0,
                    coverageType: 'indirect'
                },
                {
                    address: 'aws_nat_gateway.main',
                    type: 'aws_nat_gateway',
                    cost: 45.00,
                    confidence: 0.95,
                    coverageType: 'numeric'
                },
            ],
            symbolicReasons: {
                'aws_lambda_function': ['Usage metric "monthly_requests" not provided', 'Average duration estimated at 200ms'],
                'aws_s3_bucket': ['Storage size depends on actual data volume', 'Request counts unknown'],
            },
            unsupportedTypes: ['aws_sagemaker_endpoint', 'aws_bedrock_custom_model'],
            pricingSnapshot: {
                id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
                cloud: 'aws',
                region: 'us-east-1',
                alias: 'default',
                fetchedAt: '2026-01-17T00:00:00Z',
                source: 'aws_pricing_api',
            },
        })
    }, [])

    const handleSymbolicSelect = (resource: ResourceCost) => {
        if (resource.coverageType === 'symbolic') {
            setSelectedSymbolic(resource)
        }
    }

    return (
        <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100">
            <Header />

            {/* View toggle */}
            <div className="max-w-7xl mx-auto px-4 pt-6">
                <div className="flex gap-2">
                    <button
                        onClick={() => setView('estimate')}
                        className={`px-4 py-2 rounded-lg font-medium transition-colors ${view === 'estimate'
                                ? 'bg-primary-600 text-white'
                                : 'bg-white text-slate-600 hover:bg-slate-50'
                            }`}
                    >
                        Cost Estimate
                    </button>
                    <button
                        onClick={() => setView('diff')}
                        className={`px-4 py-2 rounded-lg font-medium transition-colors ${view === 'diff'
                                ? 'bg-primary-600 text-white'
                                : 'bg-white text-slate-600 hover:bg-slate-50'
                            }`}
                    >
                        Diff View
                    </button>
                </div>
            </div>

            <main className="max-w-7xl mx-auto px-4 py-6">
                {result && view === 'estimate' && (
                    <div className="space-y-6">
                        {/* Summary and Tree */}
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                            {/* Summary Card */}
                            <div className="lg:col-span-1">
                                <CostSummaryCard
                                    totalCost={result.totalCost}
                                    confidence={result.confidence}
                                    coverage={result.coverage}
                                />

                                {/* Snapshot info */}
                                {result.pricingSnapshot && (
                                    <div className="mt-4 bg-white rounded-xl shadow-sm border border-slate-200 p-4 text-xs">
                                        <div className="font-medium text-slate-700 mb-2">Pricing Snapshot</div>
                                        <div className="space-y-1 text-slate-500 font-mono">
                                            <div>{result.pricingSnapshot.cloud}/{result.pricingSnapshot.region}</div>
                                            <div className="truncate">{result.pricingSnapshot.id}</div>
                                            <div className="text-slate-400">{result.pricingSnapshot.source}</div>
                                        </div>
                                    </div>
                                )}
                            </div>

                            {/* Cost Tree */}
                            <div className="lg:col-span-2">
                                <CostTree
                                    resources={result.resources}
                                    onSelectSymbolic={handleSymbolicSelect}
                                />
                            </div>
                        </div>

                        {/* Symbolic Explanation */}
                        {selectedSymbolic && (
                            <SymbolicExplanationPanel
                                resource={selectedSymbolic}
                                reasons={result.symbolicReasons[selectedSymbolic.type] || ['Unknown reason']}
                                onClose={() => setSelectedSymbolic(null)}
                            />
                        )}

                        {/* Unsupported Warning */}
                        <UnsupportedWarning types={result.unsupportedTypes} />
                    </div>
                )}

                {diff && view === 'diff' && (
                    <DiffSummary diff={diff} />
                )}

                {!diff && view === 'diff' && (
                    <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-12 text-center">
                        <div className="text-slate-400 mb-2">
                            <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1}
                                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                            </svg>
                        </div>
                        <div className="text-slate-600">No diff available</div>
                        <div className="text-sm text-slate-400 mt-1">Run cost estimation on a Terraform plan to see changes</div>
                    </div>
                )}
            </main>
        </div>
    )
}

export default App
