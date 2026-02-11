import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

// Matching Go struct ClusterSummary
export interface ClusterSummary {
  totalNodes: number;
  healthyNodes: number;
  totalGPUs: number;
  allocatedGPUs: number;
  totalQueues: number;
  jobCounts: { [status: string]: number };
}

@Injectable({
  providedIn: 'root'
})
export class VisualizerService {

  private apiUrl = '/api/v1/visualizer'; // Relative path, proxied by Angular CLI

  constructor(private http: HttpClient) { }

  getClusterSummary(): Observable<ClusterSummary> {
    return this.http.get<ClusterSummary>(`${this.apiUrl}/summary`);
  }

  getJobs(namespace: string = ''): Observable<JobView[]> {
    const params: { [key: string]: string } = {};
    if (namespace) {
      params['namespace'] = namespace;
    }
    return this.http.get<JobView[]>(`${this.apiUrl}/jobs`, { params });
  }

  getNodes(): Observable<NodeView[]> {
    return this.http.get<NodeView[]>(`${this.apiUrl}/nodes`);
  }

  getQueues(): Observable<QueueView[]> {
    return this.http.get<QueueView[]>(`${this.apiUrl}/queues`);
  }
}

export interface QueueResources {
  guaranteed: ResourceStats;
  allocated: ResourceStats;
  max: ResourceStats;
}

export interface QueueView {
  name: string;
  parent: string;
  weight: number;
  resources: QueueResources;
  children: QueueView[];
}

export interface ResourceStats {
  milliCPU: number;
  memory: number;
  gpu: number;
  scalarResources?: { [key: string]: number };
}

export interface GPUSlot {
  id: number;
  occupiedBy: string;
  fragmented: boolean;
}

export interface NodeView {
  name: string;
  status: string;
  allocatable: ResourceStats;
  used: ResourceStats;
  gpuSlots: GPUSlot[];
}

export interface TaskView {
  name: string;
  status: string;
  nodeName: string;
}

export interface JobView {
  uid: string;
  name: string;
  namespace: string;
  queue: string;
  status: string;
  createTime: string;
  tasks: TaskView[];
}
