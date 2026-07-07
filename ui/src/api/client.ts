export const fetchServices = async (): Promise<string[]> => {
  const res = await fetch('http://localhost:8080/api/services');
  return res.json();
};

export const fetchGraph = async (): Promise<any> => {
  const res = await fetch('http://localhost:8080/api/graph');
  return res.json();
};
