from typing import List, Dict, Any, Optional
from disease_database import DISEASE_DATABASE

class CaseRecommender:
    def __init__(self):
        self.disease_db = DISEASE_DATABASE
        self.additional_cases = self._load_additional_cases()
    
    def _load_additional_cases(self) -> List[Dict]:
        return [
            {
                "id": "CASE-EXT-001",
                "disease_name": "稻曲病",
                "symptoms": "稻穗上出现黄色或墨绿色粉状物，病粒肿大",
                "location": "湖南省益阳市",
                "treatment": "孕穗期喷施井岗霉素或戊唑醇",
                "effectiveness": "良好",
                "lessons": "关键时期预防效果最佳"
            },
            {
                "id": "CASE-EXT-002",
                "disease_name": "番茄病毒病",
                "symptoms": "叶片皱缩、花叶、黄化，植株矮化",
                "location": "山东省寿光市",
                "treatment": "防治蚜虫，喷施病毒A或宁南霉素",
                "effectiveness": "一般",
                "lessons": "预防为主，早期防治效果好"
            },
            {
                "id": "CASE-EXT-003",
                "disease_name": "梨黑星病",
                "symptoms": "叶片和果实上产生黑色霉层，病斑凹陷",
                "location": "河北省石家庄市",
                "treatment": "落花后喷施代森锰锌或苯醚甲环唑",
                "effectiveness": "良好",
                "lessons": "连续阴雨前喷药保护"
            }
        ]
    
    def find_similar_cases(self, disease_name: str, symptoms: str = "", 
                           location: str = "", limit: int = 5) -> Dict[str, Any]:
        matched_cases = []
        
        for category, diseases in self.disease_db.items():
            for disease in diseases:
                if disease_name.lower() in disease.get('name', '').lower() or \
                   disease.get('name', '').lower() in disease_name.lower():
                    
                    similar_cases = disease.get('similar_cases', [])
                    for case in similar_cases:
                        matched_cases.append({
                            **case,
                            "disease_name": disease.get('name'),
                            "symptoms": disease.get('symptoms'),
                            "source": "disease_database"
                        })
        
        for case in self.additional_cases:
            if disease_name.lower() in case.get('disease_name', '').lower():
                matched_cases.append({
                    **case,
                    "source": "additional_cases"
                })
        
        if not matched_cases:
            matched_cases = self._find_by_symptoms(symptoms)
        
        matched_cases = matched_cases[:limit]
        
        return {
            "success": True,
            "disease_name": disease_name,
            "total_cases": len(matched_cases),
            "cases": matched_cases
        }
    
    def _find_by_symptoms(self, symptoms: str) -> List[Dict]:
        if not symptoms:
            return []
        
        symptom_keywords = self._extract_keywords(symptoms)
        matched_cases = []
        
        for category, diseases in self.disease_db.items():
            for disease in diseases:
                disease_symptoms = disease.get('symptoms', '').lower()
                match_score = 0
                
                for keyword in symptom_keywords:
                    if keyword.lower() in disease_symptoms:
                        match_score += 1
                
                if match_score > 0:
                    similar_cases = disease.get('similar_cases', [])
                    for case in similar_cases:
                        matched_cases.append({
                            **case,
                            "disease_name": disease.get('name'),
                            "symptoms": disease.get('symptoms'),
                            "match_score": match_score,
                            "source": "disease_database"
                        })
        
        matched_cases.sort(key=lambda x: x.get('match_score', 0), reverse=True)
        return matched_cases[:5]
    
    def _extract_keywords(self, text: str) -> List[str]:
        symptom_keywords = [
            "病斑", "黄化", "坏死", "腐烂", "霉层", "粉状物",
            "枯萎", "萎蔫", "畸形", "皱缩", "斑点", "条纹",
            "水渍状", "干枯", "落叶", "落果", "矮化", "丛枝"
        ]
        
        found_keywords = []
        text_lower = text.lower()
        
        for keyword in symptom_keywords:
            if keyword in text_lower:
                found_keywords.append(keyword)
        
        return found_keywords if found_keywords else text.split()[:5]
    
    def get_case_details(self, case_id: str) -> Optional[Dict]:
        for category, diseases in self.disease_db.items():
            for disease in diseases:
                for case in disease.get('similar_cases', []):
                    if case.get('id') == case_id:
                        return {
                            **case,
                            "disease_name": disease.get('name'),
                            "symptoms": disease.get('symptoms'),
                            "treatment": disease.get('recommended_actions', [])
                        }
        
        for case in self.additional_cases:
            if case.get('id') == case_id:
                return case
        
        return None
    
    def get_recommendations_by_location(self, location: str, 
                                          crop_type: str = "") -> List[Dict]:
        recommendations = []
        
        location_keywords = location.lower()
        
        for category, diseases in self.disease_db.items():
            for disease in diseases:
                for case in disease.get('similar_cases', []):
                    case_desc = case.get('description', '').lower()
                    if any(kw in case_desc for kw in location_keywords.split()):
                        recommendations.append({
                            **case,
                            "disease_name": disease.get('name'),
                            "severity": "中度",
                            "recommendation": disease.get('recommended_actions', [])
                        })
        
        return recommendations[:10]
