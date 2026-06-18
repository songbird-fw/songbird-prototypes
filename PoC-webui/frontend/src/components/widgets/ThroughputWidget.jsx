import React from 'react';
import Widget from './Widget';
import { Network } from 'lucide-react';
import { Line } from 'react-chartjs-2';

const ThroughputWidget = ({ data, options }) => {
  return (
    <Widget
      title="Interface Throughput"
      icon={Network}
      headerAction={
        <div className="flex gap-3">
          <div className="flex items-center gap-1">
            <div className="w-1.5 h-1.5 rounded-full bg-blue-500"></div>
            <span className="text-[8px] font-bold text-slate-400 uppercase">WAN In</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
            <span className="text-[8px] font-bold text-slate-400 uppercase">WAN Out</span>
          </div>
        </div>
      }
    >
      <div className="p-4 h-[140px]">
        <Line data={data} options={options} />
      </div>
    </Widget>
  );
};

export default ThroughputWidget;
