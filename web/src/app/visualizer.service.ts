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
}
