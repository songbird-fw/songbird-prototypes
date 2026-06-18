import React from 'react';
import Widget from './Widget';
import { Pie } from 'react-chartjs-2';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';

ChartJS.register(ArcElement, Tooltip, Legend);

const DistributionWidget = ({ title, icon, data }) => {
  const options = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'right',
        labels: {
          boxWidth: 6,
          usePointStyle: true,
          font: { size: 8 }
        }
      }
    }
  };

  return (
    <Widget title={title} icon={icon}>
      <div className="p-2 h-[100px] flex items-center justify-center">
        <Pie data={data} options={options} />
      </div>
    </Widget>
  );
};

export default DistributionWidget;
