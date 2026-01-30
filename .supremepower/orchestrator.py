import json
import sys
import os

def load_config():
    config_path = os.path.join(os.path.dirname(__file__), 'config.json')
    with open(config_path, 'r') as f:
        return json.load(f)

def analyze_complexity(task_description):
    # Placeholder for actual complexity analysis logic
    # In a real scenario, this might call an LLM or use heuristics
    if "architect" in task_description.lower() or "system" in task_description.lower():
        return 0.9
    return 0.5

def orchestrate(task):
    config = load_config()
    complexity = analyze_complexity(task)
    
    if complexity >= config['agents']['master_architect']['complexity_threshold']:
        print("Activating Master Architect for high-complexity task.")
        return "master_architect"
    else:
        print("Task complexity standard. Using default agent.")
        return "default"

if __name__ == "__main__":
    if len(sys.argv) > 1:
        task = sys.argv[1]
        agent = orchestrate(task)
        print(f"Orchestrator decision: {agent}")
