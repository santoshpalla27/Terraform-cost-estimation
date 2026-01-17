import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import type { ResourceCost } from '../types'

interface Props {
    resources: ResourceCost[]
}

const COLORS = {
    numeric: '#10b981',
    symbolic: '#8b5cf6',
    indirect: '#6b7280',
    unsupported: '#ef4444',
}

export function CostBreakdown({ resources }: Props) {
    const billableResources = resources
        .filter(r => r.cost > 0)
        .sort((a, b) => b.cost - a.cost)
        .slice(0, 10)

    const data = billableResources.map(r => ({
        name: r.address.split('.').pop() || r.address,
        cost: r.cost,
        type: r.coverageType,
        fullAddress: r.address,
    }))

    return (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-6">
            <h2 className="text-lg font-semibold text-slate-900 mb-4">Cost Breakdown (Top 10)</h2>

            <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={data} layout="vertical" margin={{ left: 20, right: 30 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                        <XAxis type="number" tickFormatter={(v) => `$${v}`} stroke="#64748b" />
                        <YAxis
                            dataKey="name"
                            type="category"
                            width={100}
                            stroke="#64748b"
                            fontSize={12}
                        />
                        <Tooltip
                            formatter={(value: number) => [`$${value.toFixed(2)}`, 'Cost']}
                            labelFormatter={(label) => data.find(d => d.name === label)?.fullAddress || label}
                            contentStyle={{
                                backgroundColor: '#fff',
                                border: '1px solid #e2e8f0',
                                borderRadius: '8px',
                            }}
                        />
                        <Bar dataKey="cost" radius={[0, 4, 4, 0]}>
                            {data.map((entry, index) => (
                                <Cell key={index} fill={COLORS[entry.type]} />
                            ))}
                        </Bar>
                    </BarChart>
                </ResponsiveContainer>
            </div>

            <div className="mt-4 flex gap-4 justify-center">
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-success"></div>
                    <span className="text-xs text-slate-600">Numeric</span>
                </div>
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-symbolic"></div>
                    <span className="text-xs text-slate-600">Symbolic</span>
                </div>
            </div>
        </div>
    )
}
