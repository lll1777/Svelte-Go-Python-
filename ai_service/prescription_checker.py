from typing import List, Dict, Any, Tuple
from disease_database import PESTICIDE_COMPATIBILITY

class PrescriptionChecker:
    def __init__(self):
        self.compatibility_data = PESTICIDE_COMPATIBILITY
    
    def check_compatibility(self, medications: List[str]) -> Dict[str, Any]:
        if not medications or len(medications) < 2:
            return {
                "success": True,
                "is_safe": True,
                "warnings": "",
                "suggestions": "单种药剂使用，请按照说明书正确施用。"
            }
        
        incompatibilities = []
        warnings = []
        suggestions = []
        
        for i in range(len(medications)):
            for j in range(i + 1, len(medications)):
                med_a = medications[i]
                med_b = medications[j]
                
                is_incompatible, reason = self._check_pair_incompatibility(med_a, med_b)
                
                if is_incompatible:
                    incompatibilities.append({
                        "medication_a": med_a,
                        "medication_b": med_b,
                        "reason": reason
                    })
                    warnings.append(f"{med_a} 与 {med_b} 存在配伍禁忌：{reason}")
        
        safe_combinations = []
        for i in range(len(medications)):
            for j in range(i + 1, len(medications)):
                med_a = medications[i]
                med_b = medications[j]
                
                is_safe, reason = self._check_pair_safety(med_a, med_b)
                if is_safe:
                    safe_combinations.append({
                        "medication_a": med_a,
                        "medication_b": med_b,
                        "reason": reason
                    })
        
        intervals = self._check_application_intervals(medications)
        if intervals:
            for interval_info in intervals:
                warnings.append(interval_info)
        
        if incompatibilities:
            is_safe = False
            suggestions.append("建议调整用药方案，避免混用存在配伍禁忌的药剂。")
            suggestions.append("如需同时使用，请咨询专业农技人员。")
        else:
            is_safe = True
            if safe_combinations:
                suggestions.append("所选药剂组合相对安全，请按推荐剂量施用。")
            else:
                suggestions.append("建议先进行小范围试验，确认安全后再大面积使用。")
        
        general_suggestions = self._get_general_safety_tips()
        suggestions.extend(general_suggestions)
        
        return {
            "success": True,
            "is_safe": is_safe,
            "incompatibilities": incompatibilities,
            "safe_combinations": safe_combinations,
            "warnings": "；".join(warnings) if warnings else "",
            "suggestions": "；".join(suggestions)
        }
    
    def _check_pair_incompatibility(self, med_a: str, med_b: str) -> Tuple[bool, str]:
        for pair in self.compatibility_data['incompatible_pairs']:
            a_name = pair['a']
            b_name = pair['b']
            
            if (self._name_matches(med_a, a_name) and self._name_matches(med_b, b_name)) or \
               (self._name_matches(med_a, b_name) and self._name_matches(med_b, a_name)):
                return True, pair['reason']
        
        return False, ""
    
    def _check_pair_safety(self, med_a: str, med_b: str) -> Tuple[bool, str]:
        for pair in self.compatibility_data['safe_combinations']:
            a_name = pair['a']
            b_name = pair['b']
            
            if (self._name_matches(med_a, a_name) and self._name_matches(med_b, b_name)) or \
               (self._name_matches(med_a, b_name) and self._name_matches(med_b, a_name)):
                return True, pair['reason']
        
        return False, ""
    
    def _check_application_intervals(self, medications: List[str]) -> List[str]:
        intervals = []
        interval_rules = self.compatibility_data['application_intervals']
        
        for med in medications:
            for rule_name, days in interval_rules.items():
                if self._name_matches(med, rule_name.split('与')[0]):
                    intervals.append(f"{rule_name}需间隔{days}天以上")
        
        return intervals
    
    def _name_matches(self, medication: str, keyword: str) -> bool:
        medication_lower = medication.lower()
        keyword_lower = keyword.lower()
        
        if keyword_lower in medication_lower:
            return True
        
        simplified_keywords = {
            '波尔多液': ['波尔多', 'bordeaux'],
            '石硫合剂': ['石硫', 'lime sulfur'],
            '铜制剂': ['铜', '铜制剂', 'copper'],
            '代森锰锌': ['代森锰锌', 'mancozeb'],
            '甲基托布津': ['甲基托布津', '托布津', 'thiophanate'],
            '井冈霉素': ['井冈霉素', 'jinggangmycin'],
            '吡虫啉': ['吡虫啉', 'imidacloprid'],
            '阿维菌素': ['阿维菌素', '阿维', 'abamectin'],
            '三环唑': ['三环唑', 'tricyclazole'],
            '苯醚甲环唑': ['苯醚甲环唑', 'difenoconazole'],
            '强碱性': ['强碱性', '碱性', 'alkaline']
        }
        
        for key, variations in simplified_keywords.items():
            if keyword_lower == key.lower():
                for var in variations:
                    if var in medication_lower:
                        return True
        
        return False
    
    def _get_general_safety_tips(self) -> List[str]:
        return [
            "严格按照农药标签规定的剂量、方法和安全间隔期使用",
            "避免在高温、大风、雨天施药",
            "施药时做好个人防护，穿戴防护服、口罩、手套等",
            "农药混用前建议先进行小范围试验",
            "注意农药的轮换使用，避免产生抗药性",
            "妥善保管农药，避免儿童接触"
        ]
    
    def get_medication_info(self, medication_name: str) -> Dict[str, Any]:
        from disease_database import DISEASE_DATABASE
        
        for category, diseases in DISEASE_DATABASE.items():
            for disease in diseases:
                for med in disease.get('medications', []):
                    if self._name_matches(medication_name, med.get('name', '')):
                        return {
                            "name": med.get('name'),
                            "dosage": med.get('dosage'),
                            "safety_note": med.get('safety_note'),
                            "used_for": disease.get('name')
                        }
        
        return {
            "name": medication_name,
            "info": "未找到详细信息，请查阅农药标签"
        }
