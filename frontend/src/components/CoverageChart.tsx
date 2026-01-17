import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from 'recharts'
import type { CoverageStats } from '../types'

interface Props {
    coverage: CoverageStats
}

const COLORS = {
    numeric: '#10b981',
    symbolic: '#8b5cf6',
    indirect: '#6b7280',
    unsupported: '#ef4444',
}

export function CoverageChart({ coverage }: Props) {
    const data = [
        { name: 'Numeric', value: coverage.numericPercent, color: COLORS.numeric },
        { name: 'Symbolic', value: coverage.symbolicPercent, color: COLORS.symbolic },
        { name: 'Indirect', value: coverage.indirectPercent, color: COLORS.indirect },
        { name: 'Unsupported', value: coverage.unsupportedPercent, color: COLORS.unsupported },
    ].filter(d => d.value > 0)

    return (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-6">
            <h2 className="text-lg font-semibold text-slate-900 mb-4">Coverage Analysis</h2>

            <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                        <Pie
                            data={data}
                            cx="50%"
                            cy="50%"
                            innerRadius={60}
                            outerRadius={100}
                            paddingAngle={2}
                            dataKey="value"
                        >
                            {data.map((entry, index) => (
                                <Cell key={index} fill={entry.color} />
                            ))}
                        </Pie>
                        <Tooltip
                            formatter={(value: number) => [`${value.toFixed(1)}%`, '']}
                            contentStyle={{
                                backgroundColor: '#fff',
                                border: '1px solid #e2e8f0',
                                borderRadius: '8px',
                            }}
                        />
                        <Legend />
                    </PieChart>
                </ResponsiveContainer>
            </div>

            <div className="mt-4 grid grid-cols-2 gap-4">
                <div className="text-center p-3 bg-green-50 rounded-lg">
                    <div className="text-2xl font-bold text-success">{coverage.numericPercent.toFixed(1)}%</div>
                    <div className="text-xs text-slate-600">Known Cost</div>
                </div>
                <div className="text-center p-3 bg-purple-50 rounded-lg">
                    <div className="text-2xl font-bold text-symbolic">{coverage.symbolicPercent.toFixed(1)}%</div>
                    <div className="text-xs text-slate-600">Estimated</div>
                </div>
            </div>
        </div>
    )
}
