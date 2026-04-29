import os
import json
from flask import Flask, request, jsonify
from flask_cors import CORS
from werkzeug.utils import secure_filename

from config import Config
from image_analyzer import ImageAnalyzer
from prescription_checker import PrescriptionChecker
from case_recommender import CaseRecommender

app = Flask(__name__)
app.config.from_object(Config)
CORS(app)

image_analyzer = ImageAnalyzer()
prescription_checker = PrescriptionChecker()
case_recommender = CaseRecommender()

def allowed_file(filename):
    return '.' in filename and \
           filename.rsplit('.', 1)[1].lower() in Config.ALLOWED_EXTENSIONS

@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({
        'status': 'ok',
        'service': 'agriculture-ai-service',
        'version': '1.0.0'
    })

@app.route('/api/diagnose', methods=['POST'])
def diagnose_image():
    if 'image' not in request.files:
        return jsonify({
            'success': False,
            'error': 'No image file provided'
        }), 400
    
    file = request.files['image']
    
    if file.filename == '':
        return jsonify({
            'success': False,
            'error': 'No selected file'
        }), 400
    
    if not allowed_file(file.filename):
        return jsonify({
            'success': False,
            'error': 'File type not allowed. Allowed types: ' + 
                     ', '.join(Config.ALLOWED_EXTENSIONS)
        }), 400
    
    try:
        filename = secure_filename(file.filename)
        image_data = file.read()
        
        crop_type = request.form.get('crop_type', 'rice')
        
        result = image_analyzer.analyze_image(image_data, filename, crop_type)
        
        return jsonify(result)
    
    except Exception as e:
        app.logger.error(f"Diagnosis error: {str(e)}")
        return jsonify({
            'success': False,
            'error': f'Image analysis failed: {str(e)}'
        }), 500

@app.route('/api/check-prescription', methods=['POST'])
def check_prescription():
    try:
        medications = request.get_json()
        
        if not medications or not isinstance(medications, list):
            return jsonify({
                'success': False,
                'error': 'Invalid request format. Expected list of medications.'
            }), 400
        
        result = prescription_checker.check_compatibility(medications)
        
        return jsonify(result)
    
    except Exception as e:
        app.logger.error(f"Prescription check error: {str(e)}")
        return jsonify({
            'success': False,
            'error': f'Prescription check failed: {str(e)}'
        }), 500

@app.route('/api/similar-cases', methods=['POST'])
def get_similar_cases():
    try:
        data = request.get_json()
        
        disease_name = data.get('disease_name', '')
        symptoms = data.get('symptoms', '')
        location = data.get('location', '')
        
        if not disease_name and not symptoms:
            return jsonify({
                'success': False,
                'error': 'Either disease_name or symptoms must be provided'
            }), 400
        
        result = case_recommender.find_similar_cases(
            disease_name=disease_name,
            symptoms=symptoms,
            location=location
        )
        
        return jsonify(result)
    
    except Exception as e:
        app.logger.error(f"Similar cases error: {str(e)}")
        return jsonify({
            'success': False,
            'error': f'Failed to find similar cases: {str(e)}'
        }), 500

@app.route('/api/generate-plan', methods=['POST'])
def generate_treatment_plan():
    try:
        data = request.get_json()
        
        disease_name = data.get('disease_name', '')
        severity = data.get('severity', '中度')
        crop_type = data.get('crop_type', 'rice')
        
        if not disease_name:
            return jsonify({
                'success': False,
                'error': 'disease_name is required'
            }), 400
        
        crop_category = image_analyzer._get_crop_category(crop_type)
        
        result = image_analyzer.generate_treatment_plan(
            disease_name=disease_name,
            severity=severity,
            crop_category=crop_category
        )
        
        return jsonify({
            'success': result.get('success', True),
            'treatment_plan': result
        })
    
    except Exception as e:
        app.logger.error(f"Treatment plan error: {str(e)}")
        return jsonify({
            'success': False,
            'error': f'Failed to generate treatment plan: {str(e)}'
        }), 500

@app.route('/api/disease/<disease_name>', methods=['GET'])
def get_disease_details(disease_name):
    try:
        crop_type = request.args.get('crop_type', 'rice')
        crop_category = image_analyzer._get_crop_category(crop_type)
        
        disease = image_analyzer.get_disease_details(disease_name, crop_category)
        
        if not disease:
            return jsonify({
                'success': False,
                'error': f'Disease "{disease_name}" not found'
            }), 404
        
        return jsonify({
            'success': True,
            'disease': disease
        })
    
    except Exception as e:
        app.logger.error(f"Disease details error: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.route('/api/case/<case_id>', methods=['GET'])
def get_case_details(case_id):
    try:
        case = case_recommender.get_case_details(case_id)
        
        if not case:
            return jsonify({
                'success': False,
                'error': f'Case "{case_id}" not found'
            }), 404
        
        return jsonify({
            'success': True,
            'case': case
        })
    
    except Exception as e:
        app.logger.error(f"Case details error: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.route('/api/medication/<medication_name>', methods=['GET'])
def get_medication_info(medication_name):
    try:
        info = prescription_checker.get_medication_info(medication_name)
        
        return jsonify({
            'success': True,
            'medication': info
        })
    
    except Exception as e:
        app.logger.error(f"Medication info error: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.route('/api/batch-diagnose', methods=['POST'])
def batch_diagnose():
    try:
        if 'images' not in request.files:
            return jsonify({
                'success': False,
                'error': 'No images provided'
            }), 400
        
        files = request.files.getlist('images')
        crop_type = request.form.get('crop_type', 'rice')
        
        results = []
        
        for file in files:
            if file.filename and allowed_file(file.filename):
                try:
                    filename = secure_filename(file.filename)
                    image_data = file.read()
                    
                    result = image_analyzer.analyze_image(image_data, filename, crop_type)
                    results.append({
                        'filename': filename,
                        'result': result
                    })
                except Exception as e:
                    results.append({
                        'filename': file.filename,
                        'error': str(e)
                    })
        
        return jsonify({
            'success': True,
            'total': len(results),
            'results': results
        })
    
    except Exception as e:
        app.logger.error(f"Batch diagnosis error: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.errorhandler(404)
def not_found(error):
    return jsonify({
        'success': False,
        'error': 'Endpoint not found'
    }), 404

@app.errorhandler(500)
def internal_error(error):
    return jsonify({
        'success': False,
        'error': 'Internal server error'
    }), 500

if __name__ == '__main__':
    app.run(
        host='0.0.0.0',
        port=Config.PORT,
        debug=Config.DEBUG
    )
