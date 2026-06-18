import React from 'react';
import Widget from './Widget';
import { Network } from 'lucide-react';
import { Line } from 'react-chartjs-2';

const ThroughputWidget = ({ data, options }) => {
  return (
    <Widget
      title="Throughput"
      icon={Network}
      headerAction={
        <div className="flex gap-2">
          <div className="flex items-center gap-1">
            <div className="w-1 h-1 rounded-full bg-blue-500"></div>
            <span className="text-[7px] font-bold text-slate-400 uppercase">In</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-1 h-1 rounded-full bg-green-500"></div>
            <span className="text-[7px] font-bold text-slate-400 uppercase">Out</span>
          </div>
        </div>
      }
    >
      <div className="p-2 h-[120px]">
        <Line data={data} options={options} />
      </div>
    </Widget>
  );
};

export default ThroughputWidget;
